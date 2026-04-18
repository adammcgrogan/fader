package handlers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"

	"github.com/adammcgrogan/fader/internal/db"
	"github.com/adammcgrogan/fader/internal/middleware"
	"github.com/adammcgrogan/fader/internal/validate"
	"github.com/google/uuid"
)

type EditHandler struct {
	db *db.Queries
}

func NewEditHandler(q *db.Queries) *EditHandler {
	return &EditHandler{db: q}
}

func (h *EditHandler) ShowEdit(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)
	renderEditPage(w, r, h.db, userID)
}

func renderEditPage(w http.ResponseWriter, r *http.Request, q *db.Queries, userID uuid.UUID) {
	profileIDStr := r.URL.Query().Get("profile")

	var profileID uuid.UUID
	if profileIDStr != "" {
		pid, err := uuid.Parse(profileIDStr)
		if err != nil {
			http.Error(w, "invalid profile", http.StatusBadRequest)
			return
		}
		profileID = pid
	} else {
		profiles, err := q.GetProfilesByUserID(r.Context(), userID)
		if err != nil || len(profiles) == 0 {
			http.Error(w, "no profiles", http.StatusInternalServerError)
			return
		}
		profileID = profiles[0].ID
	}

	profile, err := q.GetProfileByID(r.Context(), profileID)
	if err != nil || profile.UserID != userID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	blocks, _ := q.GetBlocksByProfileID(r.Context(), profileID)
	allProfiles, _ := q.GetProfilesByUserID(r.Context(), userID)
	user, _ := q.GetUserByID(r.Context(), userID)

	renderTemplate(w, "edit.html", map[string]any{
		"Profile":     profile,
		"Blocks":      blocks,
		"AllProfiles": allProfiles,
		"User":        user,
	})
}

func (h *EditHandler) AddBlock(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	profileID, err := uuid.Parse(r.FormValue("profile_id"))
	if err != nil {
		http.Error(w, "invalid profile", http.StatusBadRequest)
		return
	}

	profile, err := h.db.GetProfileByID(r.Context(), profileID)
	if err != nil || profile.UserID != userID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	blockType := r.FormValue("type")
	data := defaultBlockData(blockType)

	block, err := h.db.CreateBlock(r.Context(), profileID, blockType, data)
	if err != nil {
		http.Error(w, "could not create block", http.StatusInternalServerError)
		return
	}

	renderPartial(w, "editable_block", block)
}

func (h *EditHandler) UpdateBlock(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)
	blockIDStr := strings.TrimPrefix(r.URL.Path, "/blocks/")
	blockID, err := uuid.Parse(blockIDStr)
	if err != nil {
		http.Error(w, "invalid block", http.StatusBadRequest)
		return
	}

	block, err := h.db.GetBlockByID(r.Context(), blockID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	profile, err := h.db.GetProfileByID(r.Context(), block.ProfileID)
	if err != nil || profile.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	data := formToBlockData(block.Type, r)
	if err := h.db.UpdateBlockData(r.Context(), blockID, data); err != nil {
		http.Error(w, "could not update block", http.StatusInternalServerError)
		return
	}

	block.Data = data
	renderPartial(w, "editable_block", block)
}

func (h *EditHandler) DeleteBlock(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)
	blockIDStr := strings.TrimPrefix(r.URL.Path, "/blocks/")
	blockID, err := uuid.Parse(blockIDStr)
	if err != nil {
		http.Error(w, "invalid block", http.StatusBadRequest)
		return
	}

	block, err := h.db.GetBlockByID(r.Context(), blockID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	profile, err := h.db.GetProfileByID(r.Context(), block.ProfileID)
	if err != nil || profile.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	h.db.DeleteBlock(r.Context(), blockID)
	w.WriteHeader(http.StatusOK)
}

func (h *EditHandler) ReorderBlocks(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Expect: block_order[]=id1&block_order[]=id2...
	ids := r.Form["block_order[]"]
	positions := make(map[uuid.UUID]int, len(ids))

	for i, idStr := range ids {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		// Verify ownership on first block only (all blocks belong to same profile)
		if i == 0 {
			block, err := h.db.GetBlockByID(r.Context(), id)
			if err != nil {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			profile, err := h.db.GetProfileByID(r.Context(), block.ProfileID)
			if err != nil || profile.UserID != userID {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		positions[id] = i
	}

	if err := h.db.UpdateBlockPositions(r.Context(), positions); err != nil {
		http.Error(w, "could not reorder", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *EditHandler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	profileID, err := uuid.Parse(r.FormValue("profile_id"))
	if err != nil {
		http.Error(w, "invalid profile", http.StatusBadRequest)
		return
	}

	profile, err := h.db.GetProfileByID(r.Context(), profileID)
	if err != nil || profile.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	tmpl := r.FormValue("template")
	validTemplates := map[string]bool{"minimal": true, "dark": true, "neon": true}
	if !validTemplates[tmpl] {
		http.Error(w, "invalid template", http.StatusBadRequest)
		return
	}

	h.db.UpdateProfileTemplate(r.Context(), profileID, tmpl)
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func (h *EditHandler) UpdateProfileInfo(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	profileID, err := uuid.Parse(r.FormValue("profile_id"))
	if err != nil {
		http.Error(w, "invalid profile", http.StatusBadRequest)
		return
	}

	profile, err := h.db.GetProfileByID(r.Context(), profileID)
	if err != nil || profile.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	displayName := r.FormValue("display_name")
	bio := r.FormValue("bio")
	var bioPtr *string
	if bio != "" {
		bioPtr = &bio
	}

	h.db.UpdateProfile(r.Context(), profileID, displayName, bioPtr, profile.AvatarURL)
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func (h *EditHandler) UpdateHandle(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	profileID, err := uuid.Parse(r.FormValue("profile_id"))
	if err != nil {
		http.Error(w, "invalid profile", http.StatusBadRequest)
		return
	}

	profile, err := h.db.GetProfileByID(r.Context(), profileID)
	if err != nil || profile.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	handle := strings.ToLower(strings.TrimSpace(r.FormValue("handle")))
	if err := validate.Handle(handle); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if handle != profile.Handle {
		exists, _ := h.db.HandleExists(r.Context(), handle)
		if exists {
			http.Error(w, "handle already taken", http.StatusConflict)
			return
		}
		h.db.UpdateProfileHandle(r.Context(), profileID, handle)
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func (h *EditHandler) NewProfile(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)

	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}

	if !user.IsPro() {
		http.Error(w, "upgrade to Pro to add more personas", http.StatusForbidden)
		return
	}

	handle := generateHandle()
	for {
		exists, _ := h.db.HandleExists(r.Context(), handle)
		if !exists {
			break
		}
		handle = generateHandle()
	}

	profile, err := h.db.CreateProfile(r.Context(), userID, handle, handle)
	if err != nil {
		http.Error(w, "could not create profile", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/edit?profile="+profile.ID.String(), http.StatusSeeOther)
}

// ── Helpers ────────────────────────────────────────────────────────────────

func defaultBlockData(blockType string) json.RawMessage {
	defaults := map[string]any{
		"social":      map[string]any{"platform": "instagram", "url": ""},
		"music_link":  map[string]any{"title": "", "url": "", "platform": "soundcloud"},
		"gig":         map[string]any{"date": "", "venue": "", "location": "", "ticket_url": ""},
		"bio":         map[string]any{"text": ""},
		"custom_link": map[string]any{"label": "", "url": ""},
		"image":       map[string]any{"url": "", "caption": ""},
		"video_link":  map[string]any{"title": "", "url": ""},
	}
	d, _ := json.Marshal(defaults[blockType])
	return d
}

func formToBlockData(blockType string, r *http.Request) json.RawMessage {
	var data map[string]any
	switch blockType {
	case "social":
		data = map[string]any{"platform": r.FormValue("platform"), "url": r.FormValue("url")}
	case "music_link":
		data = map[string]any{"title": r.FormValue("title"), "url": r.FormValue("url"), "platform": r.FormValue("platform")}
	case "gig":
		data = map[string]any{"date": r.FormValue("date"), "venue": r.FormValue("venue"), "location": r.FormValue("location"), "ticket_url": r.FormValue("ticket_url")}
	case "bio":
		data = map[string]any{"text": r.FormValue("text")}
	case "custom_link":
		data = map[string]any{"label": r.FormValue("label"), "url": r.FormValue("url")}
	case "image":
		data = map[string]any{"url": r.FormValue("url"), "caption": r.FormValue("caption")}
	case "video_link":
		data = map[string]any{"title": r.FormValue("title"), "url": r.FormValue("url")}
	default:
		data = map[string]any{}
	}
	b, _ := json.Marshal(data)
	return b
}

func (h *EditHandler) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)

	profileID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid profile id", http.StatusBadRequest)
		return
	}

	profile, err := h.db.GetProfileByID(r.Context(), profileID)
	if err != nil || profile.UserID != userID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	profiles, _ := h.db.GetProfilesByUserID(r.Context(), userID)
	if len(profiles) <= 1 {
		http.Error(w, "cannot delete your only page", http.StatusBadRequest)
		return
	}

	h.db.DeleteProfile(r.Context(), profileID)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func generateHandle() string {
	return fmt.Sprintf("page%04d", rand.Intn(10000))
}
