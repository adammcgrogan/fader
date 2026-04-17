package handlers

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"

	"github.com/adammcgrogan/fader/internal/db"
	"github.com/adammcgrogan/fader/internal/middleware"
	"github.com/google/uuid"
)

type ProfileHandler struct {
	db *db.Queries
}

func NewProfileHandler(q *db.Queries) *ProfileHandler {
	return &ProfileHandler{db: q}
}

func (h *ProfileHandler) ServeProfile(w http.ResponseWriter, r *http.Request) {
	handle := middleware.GetSubdomain(r)
	if handle == "" {
		http.NotFound(w, r)
		return
	}

	profile, err := h.db.GetProfileByHandle(r.Context(), handle)
	if err != nil {
		renderTemplate(w, "claimed.html", map[string]any{"Handle": handle})
		return
	}

	blocks, _ := h.db.GetBlocksByProfileID(r.Context(), profile.ID)

	// Record view — skip if the viewer is the owner
	viewerID, _ := middleware.GetUserID(r)
	if viewerID != profile.UserID {
		go h.db.RecordEvent(
			r.Context(),
			profile.ID,
			nil,
			"view",
			hashIP(r.RemoteAddr),
			r.Header.Get("CF-IPCountry"),
			r.Referer(),
			r.UserAgent(),
		)
	}

	renderTemplate(w, "profile_"+profile.Template+".html", map[string]any{
		"Profile": profile,
		"Blocks":  blocks,
	})
}

// Redirect handles /r/:blockID — records click then redirects to the block URL.
func (h *ProfileHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	blockIDStr := strings.TrimPrefix(r.URL.Path, "/r/")
	blockID, err := uuid.Parse(blockIDStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	block, err := h.db.GetBlockByID(r.Context(), blockID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	url := extractURL(block.Data)
	if url == "" {
		http.NotFound(w, r)
		return
	}

	go h.db.RecordEvent(
		r.Context(),
		block.ProfileID,
		&blockID,
		"click",
		hashIP(r.RemoteAddr),
		r.Header.Get("CF-IPCountry"),
		r.Referer(),
		r.UserAgent(),
	)

	http.Redirect(w, r, url, http.StatusFound)
}

func hashIP(remote string) string {
	// Strip port
	if idx := strings.LastIndex(remote, ":"); idx != -1 {
		remote = remote[:idx]
	}
	h := sha256.Sum256([]byte(remote))
	return fmt.Sprintf("%x", h)
}

func extractURL(data []byte) string {
	// Quick JSON extract without full unmarshal for the common {"url":"..."} pattern
	s := string(data)
	start := strings.Index(s, `"url":"`)
	if start == -1 {
		return ""
	}
	start += 7
	end := strings.Index(s[start:], `"`)
	if end == -1 {
		return ""
	}
	return s[start : start+end]
}
