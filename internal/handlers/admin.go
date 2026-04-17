package handlers

import (
	"net/http"
	"strings"

	"github.com/adammcgrogan/fader/internal/db"
	"github.com/adammcgrogan/fader/internal/middleware"
	"github.com/google/uuid"
)

type AdminHandler struct {
	db          *db.Queries
	adminUserID string
}

func NewAdminHandler(q *db.Queries, adminUserID string) *AdminHandler {
	return &AdminHandler{db: q, adminUserID: adminUserID}
}

func (h *AdminHandler) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.GetUserID(r)
		if !ok || userID.String() != h.adminUserID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	users, err := h.db.ListAllUsers(r.Context())
	if err != nil {
		http.Error(w, "could not load users", http.StatusInternalServerError)
		return
	}
	profiles, err := h.db.ListAllProfiles(r.Context())
	if err != nil {
		http.Error(w, "could not load profiles", http.StatusInternalServerError)
		return
	}
	renderTemplate(w, "admin.html", map[string]any{
		"Users":    users,
		"Profiles": profiles,
	})
}

func (h *AdminHandler) EditProfile(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/admin/profiles/")
	idStr = strings.TrimSuffix(idStr, "/edit")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	profile, err := h.db.GetProfileByID(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	blocks, _ := h.db.GetBlocksByProfileID(r.Context(), id)
	user, _ := h.db.GetUserByID(r.Context(), profile.UserID)

	renderTemplate(w, "admin_profile_edit.html", map[string]any{
		"Profile": profile,
		"Blocks":  blocks,
		"Owner":   user,
	})
}

func (h *AdminHandler) SetVerified(w http.ResponseWriter, r *http.Request) {
	// Verified status can be stored as a profile field — for now toggle via admin
	// This is a placeholder for when the verified column is added in a future migration
	w.WriteHeader(http.StatusNotImplemented)
}

func (h *AdminHandler) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/admin/profiles/")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	h.db.DeleteProfile(r.Context(), id)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *AdminHandler) SetUserTier(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(r.FormValue("user_id"))
	if err != nil {
		http.Error(w, "invalid user", http.StatusBadRequest)
		return
	}

	tier := r.FormValue("tier")
	if tier != "free" && tier != "pro" {
		http.Error(w, "invalid tier", http.StatusBadRequest)
		return
	}

	h.db.SetUserTier(r.Context(), userID, tier)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}
