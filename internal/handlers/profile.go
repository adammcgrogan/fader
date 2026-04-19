package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
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

	if r.URL.Path == "/" {
		profileID := profile.ID
		ip := hashIP(r.RemoteAddr)
		country := r.Header.Get("CF-IPCountry")
		if country == "" {
			country = "Unknown"
		}
		referrer := r.Referer()
		ua := r.UserAgent()
		go h.db.RecordEvent(
			context.Background(),
			profileID,
			nil,
			"view",
			ip,
			country,
			referrer,
			ua,
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

	url := extractURL(block.Type, block.Data)
	if url == "" {
		http.NotFound(w, r)
		return
	}

	pid := block.ProfileID
	bid := blockID
	ip := hashIP(r.RemoteAddr)
	country := r.Header.Get("CF-IPCountry")
	referrer := r.Referer()
	ua := r.UserAgent()
	go h.db.RecordEvent(
		context.Background(),
		pid,
		&bid,
		"click",
		ip,
		country,
		referrer,
		ua,
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

func extractURL(blockType string, data []byte) string {
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	if blockType == "ra_link" {
		if u := m["username"]; u != "" {
			return "https://ra.co/dj/" + u
		}
		return ""
	}
	return m["url"]
}
