package main

import (
	"context"
	"log"
	"net/http"
	"os"

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

	auth := handlers.NewAuthHandler(queries, supa.Auth)
	profile := handlers.NewProfileHandler(queries)
	edit := handlers.NewEditHandler(queries)
	dashboard := handlers.NewDashboardHandler(queries)
	analytics := handlers.NewAnalyticsHandler(queries)
	stripeH := handlers.NewStripeHandler(queries, cfg)
	admin := handlers.NewAdminHandler(queries, cfg.AdminUserID)

	authMW, err := middleware.NewAuthMiddleware(cfg.SupabaseURL)
	if err != nil {
		log.Fatalf("auth middleware: %v", err)
	}
	requireAuth := authMW.RequireAuth
	optionalAuth := authMW.OptionalAuth

	mux := http.NewServeMux()

	// Static assets
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Auth routes
	mux.HandleFunc("GET /auth/login", auth.ShowLogin)
	mux.HandleFunc("POST /auth/login", auth.Login)
	mux.HandleFunc("GET /auth/register", auth.ShowRegister)
	mux.HandleFunc("POST /auth/register", auth.Register)
	mux.HandleFunc("POST /auth/logout", auth.Logout)

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
	mux.Handle("DELETE /profiles/{id}", requireAuth(http.HandlerFunc(edit.DeleteProfile)))

	// Analytics
	mux.Handle("GET /analytics", requireAuth(http.HandlerFunc(analytics.Show)))

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
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
