package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	supabase "github.com/supabase-community/supabase-go"

	"github.com/adammcgrogan/fader/internal/config"
	"github.com/adammcgrogan/fader/internal/db"
	"github.com/adammcgrogan/fader/internal/handlers"
	"github.com/adammcgrogan/fader/internal/middleware"
)

func main() {
	godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(context.Background(), pool, "migrations"); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	queries := db.New(pool)

	supa, err := supabase.NewClient(cfg.SupabaseURL, cfg.SupabaseAnonKey, &supabase.ClientOptions{})
	if err != nil {
		log.Fatalf("supabase client: %v", err)
	}

	handlers.BaseDomain = cfg.BaseDomain
	handlers.LoadTemplates("templates")
	handlers.LoadChangelog()

	auth := handlers.NewAuthHandler(queries, supa.Auth)
	profile := handlers.NewProfileHandler(queries)
	edit := handlers.NewEditHandler(queries, cfg.SupabaseURL)
	dashboard := handlers.NewDashboardHandler(queries)
	analytics := handlers.NewAnalyticsHandler(queries)
	inquiries := handlers.NewInquiriesHandler(queries)
	stripeH := handlers.NewStripeHandler(queries, cfg)
	if cfg.AdminUserID == "" {
		log.Println("warning: ADMIN_USER_ID not set — admin panel is inaccessible")
	}
	admin := handlers.NewAdminHandler(queries, cfg.AdminUserID)
	discover := handlers.NewDiscoverHandler(queries)

	authMW, err := middleware.NewAuthMiddleware(cfg.SupabaseURL, supa.Auth, cfg.BaseDomain)
	if err != nil {
		log.Fatalf("auth middleware: %v", err)
	}
	requireAuth := authMW.RequireAuth
	optionalAuth := authMW.OptionalAuth

	mux := http.NewServeMux()

	// Static assets
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Health checks — liveness is cheap, readiness pings the DB.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	// Auth routes
	mux.HandleFunc("GET /auth/login", auth.ShowLogin)
	mux.HandleFunc("POST /auth/login", auth.Login)
	mux.HandleFunc("GET /auth/register", auth.ShowRegister)
	mux.HandleFunc("POST /auth/register", auth.Register)
	mux.HandleFunc("POST /auth/logout", auth.Logout)
	mux.HandleFunc("GET /auth/forgot-password", auth.ShowForgotPassword)
	mux.HandleFunc("POST /auth/forgot-password", auth.ForgotPassword)
	mux.HandleFunc("GET /auth/reset-password", auth.ShowResetPassword)
	mux.HandleFunc("POST /auth/set-password", auth.SetPassword)
	mux.HandleFunc("GET /handles/check", auth.CheckHandle)
	mux.HandleFunc("GET /changelog", handlers.Changelog)
	mux.HandleFunc("GET /discover", discover.Show)

	// Outbound click tracking (public)
	mux.HandleFunc("/r/", profile.Redirect)

	// Catch-all: landing page or DJ profile based on subdomain
	mux.Handle("/", optionalAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub := middleware.GetSubdomain(r)
		switch sub {
		case "", "www":
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			handlers.ServeLanding(w, r)
		default:
			profile.ServeProfile(w, r)
		}
	})))

	// Dashboard
	mux.Handle("GET /dashboard", requireAuth(http.HandlerFunc(dashboard.Show)))
	mux.Handle("GET /inquiries", requireAuth(http.HandlerFunc(inquiries.Show)))

	// Edit
	mux.Handle("GET /edit", requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		edit.ShowEdit(w, r)
	})))
	mux.Handle("POST /profiles/new", requireAuth(http.HandlerFunc(edit.NewProfile)))
	mux.Handle("POST /blocks", requireAuth(http.HandlerFunc(edit.AddBlock)))
	mux.Handle("PATCH /blocks/order", requireAuth(http.HandlerFunc(edit.ReorderBlocks)))
	mux.Handle("PUT /blocks/{id}", requireAuth(http.HandlerFunc(edit.UpdateBlock)))
	mux.Handle("DELETE /blocks/{id}", requireAuth(http.HandlerFunc(edit.DeleteBlock)))
	mux.Handle("PATCH /profile/template", requireAuth(http.HandlerFunc(edit.UpdateTemplate)))
	mux.Handle("PATCH /profile/info", requireAuth(http.HandlerFunc(edit.UpdateProfileInfo)))
	mux.Handle("PATCH /profile/handle", requireAuth(http.HandlerFunc(edit.UpdateHandle)))
	mux.Handle("POST /profile/avatar", requireAuth(http.HandlerFunc(edit.UploadAvatar)))
	mux.Handle("PATCH /profile/genres", requireAuth(http.HandlerFunc(edit.UpdateGenres)))
	mux.Handle("PATCH /profile/discover", requireAuth(http.HandlerFunc(edit.UpdateDiscoverSettings)))
	mux.Handle("PATCH /profile/footer", requireAuth(http.HandlerFunc(edit.UpdateFooterSettings)))
	mux.Handle("DELETE /profiles/{id}", requireAuth(http.HandlerFunc(edit.DeleteProfile)))

	// Analytics
	mux.Handle("GET /analytics", requireAuth(http.HandlerFunc(analytics.Show)))

	// Public inquiries
	mux.HandleFunc("POST /inquiries", inquiries.Submit)

	// Billing
	mux.Handle("GET /billing/checkout", requireAuth(http.HandlerFunc(stripeH.Checkout)))
	mux.Handle("GET /billing/portal", requireAuth(http.HandlerFunc(stripeH.Portal)))
	mux.Handle("GET /billing/success", requireAuth(http.HandlerFunc(stripeH.BillingSuccess)))
	mux.Handle("POST /webhooks/stripe", http.HandlerFunc(stripeH.Webhook))

	// Admin (superadmin only)
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /admin", admin.Dashboard)
	adminMux.HandleFunc("GET /admin/profiles/{id}/edit", admin.EditProfile)
	adminMux.HandleFunc("DELETE /admin/profiles/{id}", admin.DeleteProfile)
	adminMux.HandleFunc("POST /admin/users/tier", admin.SetUserTier)
	mux.Handle("/admin", requireAuth(admin.RequireAdmin(adminMux)))
	mux.Handle("/admin/", requireAuth(admin.RequireAdmin(adminMux)))

	// Wrap everything in subdomain extraction + method override
	handler := middleware.SubdomainFromHeader(cfg.BaseDomain)(middleware.MethodOverride(mux))

	addr := ":" + cfg.Port
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// Purge analytics events older than 90 days, once at startup then every 24h
	go func() {
		purge := func() {
			n, err := queries.PurgeOldAnalyticsEvents(context.Background())
			if err != nil {
				log.Printf("analytics purge error: %v", err)
			} else if n > 0 {
				log.Printf("analytics purge: removed %d rows older than 90 days", n)
			}
		}
		purge()
		for range time.Tick(24 * time.Hour) {
			purge()
		}
	}()

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		log.Fatalf("server error: %v", err)
	case sig := <-stop:
		log.Printf("received %s, shutting down", sig)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Printf("shutdown complete")
}
