package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/adammcgrogan/fader/internal/db"
	"github.com/adammcgrogan/fader/internal/middleware"
	"github.com/adammcgrogan/fader/internal/models"
	"github.com/google/uuid"
)

type AnalyticsHandler struct {
	db *db.Queries
}

func NewAnalyticsHandler(q *db.Queries) *AnalyticsHandler {
	return &AnalyticsHandler{db: q}
}

func (h *AnalyticsHandler) Show(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserID(r)

	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}

	if !user.IsPro() {
		renderTemplate(w, "analytics_upgrade.html", map[string]any{"User": user})
		return
	}

	days := 30
	if d, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil {
		switch d {
		case 7, 30, 90:
			days = d
		}
	}

	profileIDStr := r.URL.Query().Get("profile")
	var profileID uuid.UUID

	if profileIDStr != "" {
		profileID, err = uuid.Parse(profileIDStr)
		if err != nil {
			http.Error(w, "invalid profile", http.StatusBadRequest)
			return
		}
	} else {
		profiles, err := h.db.GetProfilesByUserID(r.Context(), userID)
		if err != nil || len(profiles) == 0 {
			http.Error(w, "no profiles", http.StatusInternalServerError)
			return
		}
		profileID = profiles[0].ID
	}

	profile, err := h.db.GetProfileByID(r.Context(), profileID)
	if err != nil || profile.UserID != userID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	summary, err := h.db.GetAnalyticsSummary(r.Context(), profileID, days)
	if err != nil {
		log.Printf("analytics summary error: %v", err)
		http.Error(w, "could not load analytics", http.StatusInternalServerError)
		return
	}

	allProfiles, _ := h.db.GetProfilesByUserID(r.Context(), userID)

	data := map[string]any{
		"Profile":     profile,
		"Summary":     summary,
		"AllProfiles": allProfiles,
		"User":        user,
		"Days":        days,
		"ChartLabels": template.JS(mustJSON(chartLabels(summary))),
		"ChartViews":  template.JS(mustJSON(chartValues(summary.ViewsByDay))),
		"ChartClicks": template.JS(mustJSON(chartValues(summary.ClicksByDay))),
	}

	if r.Header.Get("HX-Request") == "true" {
		renderPartial(w, "analytics_content", data)
		return
	}
	renderTemplate(w, "analytics.html", data)
}

func chartLabels(s *models.AnalyticsSummary) []string {
	seen := map[string]bool{}
	for _, d := range s.ViewsByDay {
		seen[d.Date] = true
	}
	for _, d := range s.ClicksByDay {
		seen[d.Date] = true
	}
	labels := make([]string, 0, len(seen))
	for k := range seen {
		labels = append(labels, k)
	}
	// sort ascending
	for i := 0; i < len(labels); i++ {
		for j := i + 1; j < len(labels); j++ {
			if labels[i] > labels[j] {
				labels[i], labels[j] = labels[j], labels[i]
			}
		}
	}
	return labels
}

func chartValues(stats []models.DailyStat) map[string]int {
	m := map[string]int{}
	for _, s := range stats {
		m[s.Date] = s.Views
	}
	return m
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
