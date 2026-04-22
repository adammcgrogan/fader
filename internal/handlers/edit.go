package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"

	"github.com/adammcgrogan/fader/internal/db"
	"github.com/adammcgrogan/fader/internal/middleware"
	"github.com/adammcgrogan/fader/internal/validate"
	"github.com/google/uuid"
)

type EditHandler struct {
	db          *db.Queries
	supabaseURL string
}

func NewEditHandler(q *db.Queries, supabaseURL string) *EditHandler {
	return &EditHandler{db: q, supabaseURL: supabaseURL}
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
	if blockType == "book_me" {
		user, err := h.db.GetUserByID(r.Context(), userID)
		if err != nil {
			http.Error(w, "user not found", http.StatusInternalServerError)
			return
		}
		if !user.IsPro() {
			http.Error(w, "upgrade to Pro to add booking inquiries", http.StatusForbidden)
			return
		}
	}
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
	if len(ids) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	firstID, err := uuid.Parse(ids[0])
	if err != nil {
		http.Error(w, "invalid block id", http.StatusBadRequest)
		return
	}
	firstBlock, err := h.db.GetBlockByID(r.Context(), firstID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	profile, err := h.db.GetProfileByID(r.Context(), firstBlock.ProfileID)
	if err != nil || profile.UserID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Build the set of block IDs the caller is allowed to reorder.
	owned, err := h.db.GetBlocksByProfileID(r.Context(), profile.ID)
	if err != nil {
		http.Error(w, "could not load blocks", http.StatusInternalServerError)
		return
	}
	ownedSet := make(map[uuid.UUID]struct{}, len(owned))
	for _, b := range owned {
		ownedSet[b.ID] = struct{}{}
	}

	positions := make(map[uuid.UUID]int, len(ids))
	for i, idStr := range ids {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		if _, ok := ownedSet[id]; !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
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

	if err := h.db.UpdateProfileTemplate(r.Context(), profileID, tmpl); err != nil {
		http.Error(w, "could not update theme", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Theme saved"))
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
		"social":      map[string]any{"platform": "", "url": ""},
		"music_link":  map[string]any{"title": "", "url": "", "platform": ""},
		"gig":         map[string]any{"date": "", "venue": "", "location": "", "ticket_url": ""},
		"bio":         map[string]any{"text": ""},
		"custom_link": map[string]any{"label": "", "url": ""},
		"image":       map[string]any{"url": "", "caption": ""},
		"video_link":  map[string]any{"title": "", "url": ""},
		"audio_embed": map[string]any{"url": "", "title": ""},
		"ra_link":     map[string]any{"username": ""},
		"residency":   map[string]any{"venue": "", "location": "", "frequency": "", "since": ""},
		"book_me":     map[string]any{"label": "Book Me", "intro_text": "Send a booking inquiry.", "submit_text": "Send inquiry"},
	}
	d, _ := json.Marshal(defaults[blockType])
	return d
}

func formToBlockData(blockType string, r *http.Request) json.RawMessage {
	var data map[string]any
	switch blockType {
	case "social":
		u := strings.TrimSpace(r.FormValue("url"))
		platform := strings.TrimSpace(r.FormValue("platform"))
		if platform == "" {
			platform = detectPlatform(u)
		}
		data = map[string]any{"platform": platform, "url": u}
	case "music_link":
		u := strings.TrimSpace(r.FormValue("url"))
		platform := strings.TrimSpace(r.FormValue("platform"))
		if platform == "" {
			platform = detectPlatform(u)
		}
		data = map[string]any{"title": r.FormValue("title"), "url": u, "platform": platform}
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
	case "audio_embed":
		data = map[string]any{"url": r.FormValue("url"), "title": r.FormValue("title")}
	case "ra_link":
		data = map[string]any{"username": strings.TrimSpace(r.FormValue("username"))}
	case "residency":
		data = map[string]any{"venue": r.FormValue("venue"), "location": r.FormValue("location"), "frequency": r.FormValue("frequency"), "since": r.FormValue("since")}
	case "book_me":
		data = map[string]any{
			"label":       strings.TrimSpace(r.FormValue("label")),
			"intro_text":  strings.TrimSpace(r.FormValue("intro_text")),
			"submit_text": strings.TrimSpace(r.FormValue("submit_text")),
		}
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

func (h *EditHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)

	if err := r.ParseMultipartForm(5 << 20); err != nil {
		http.Error(w, "file too large (max 5MB)", http.StatusBadRequest)
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

	file, header, err := r.FormFile("avatar")
	if err != nil {
		http.Error(w, "no file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Detect actual content type from file bytes
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	detectedType := http.DetectContentType(buf[:n])
	if _, err := (file.(io.ReadSeeker)).Seek(0, io.SeekStart); err != nil {
		http.Error(w, "upload error", http.StatusInternalServerError)
		return
	}

	// Prefer client-declared type for webp (DetectContentType can't identify it)
	contentType := detectedType
	if ct := header.Header.Get("Content-Type"); ct == "image/webp" {
		contentType = ct
	}

	allowed := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
	}
	if !allowed[contentType] {
		http.Error(w, "unsupported image type — use JPEG, PNG, WebP or GIF", http.StatusBadRequest)
		return
	}

	cookie, err := r.Cookie("sb-token")
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	storagePath := fmt.Sprintf("avatars/%s-avatar", profileID.String())
	uploadURL := strings.TrimRight(h.supabaseURL, "/") + "/storage/v1/object/" + storagePath

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, uploadURL, file)
	if err != nil {
		http.Error(w, "upload error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+cookie.Value)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-upsert", "true")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "storage upload failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		http.Error(w, "storage error: "+string(body), http.StatusInternalServerError)
		return
	}

	publicURL := strings.TrimRight(h.supabaseURL, "/") + "/storage/v1/object/public/" + storagePath
	h.db.UpdateProfile(r.Context(), profileID, profile.DisplayName, profile.Bio, &publicURL)

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func (h *EditHandler) UpdateGenres(w http.ResponseWriter, r *http.Request) {
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

	var genres []string
	for _, g := range strings.Split(r.FormValue("genres"), ",") {
		g = strings.TrimSpace(g)
		if g != "" {
			genres = append(genres, g)
		}
	}

	h.db.UpdateProfileGenres(r.Context(), profileID, genres)
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func (h *EditHandler) UpdateDiscoverSettings(w http.ResponseWriter, r *http.Request) {
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

	hidden := r.FormValue("discover_hidden") == "on"
	if err := h.db.UpdateDiscoverHidden(r.Context(), profileID, hidden); err != nil {
		http.Error(w, "could not update settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func (h *EditHandler) UpdateFooterSettings(w http.ResponseWriter, r *http.Request) {
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

	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}
	if !user.IsPro() {
		http.Error(w, "upgrade to Pro to hide the footer", http.StatusForbidden)
		return
	}

	hideFooter := r.FormValue("hide_footer") == "on"
	if err := h.db.UpdateProfileHideFooter(r.Context(), profileID, hideFooter); err != nil {
		http.Error(w, "could not update footer settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func generateHandle() string {
	return fmt.Sprintf("page%04d", rand.Intn(10000))
}

// detectPlatform returns a canonical platform slug for a URL, or "" if unknown.
// Matches by exact suffix on the parsed host so lookalike domains can't spoof.
func detectPlatform(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.ToLower(u.Host)
	if i := strings.Index(host, ":"); i != -1 {
		host = host[:i]
	}
	host = strings.TrimPrefix(host, "www.")

	platforms := []struct {
		domain, name string
	}{
		{"instagram.com", "instagram"},
		{"tiktok.com", "tiktok"},
		{"twitter.com", "twitter"},
		{"x.com", "twitter"},
		{"facebook.com", "facebook"},
		{"fb.com", "facebook"},
		{"youtube.com", "youtube"},
		{"youtu.be", "youtube"},
		{"twitch.tv", "twitch"},
		{"threads.net", "threads"},
		{"soundcloud.com", "soundcloud"},
		{"spotify.com", "spotify"},
		{"open.spotify.com", "spotify"},
		{"mixcloud.com", "mixcloud"},
		{"bandcamp.com", "bandcamp"},
		{"beatport.com", "beatport"},
		{"apple.com", "apple music"},
		{"music.apple.com", "apple music"},
		{"tidal.com", "tidal"},
		{"deezer.com", "deezer"},
		{"ra.co", "resident advisor"},
		{"residentadvisor.net", "resident advisor"},
		{"bandsintown.com", "bandsintown"},
		{"linkedin.com", "linkedin"},
		{"discord.gg", "discord"},
		{"discord.com", "discord"},
		{"patreon.com", "patreon"},
		{"linktr.ee", "linktree"},
	}
	for _, p := range platforms {
		if host == p.domain || strings.HasSuffix(host, "."+p.domain) {
			return p.name
		}
	}
	return ""
}
