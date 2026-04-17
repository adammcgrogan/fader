package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"github.com/adammcgrogan/fader/internal/models"
)

var templates *template.Template

var funcMap = template.FuncMap{
	"unmarshalBio":        unmarshalFn[models.BioData],
	"unmarshalSocial":     unmarshalFn[models.SocialData],
	"unmarshalMusicLink":  unmarshalFn[models.MusicLinkData],
	"unmarshalGig":        unmarshalFn[models.GigData],
	"unmarshalCustomLink": unmarshalFn[models.CustomLinkData],
	"unmarshalImage":      unmarshalFn[models.ImageData],
	"unmarshalVideoLink":  unmarshalFn[models.VideoLinkData],
	"list": func(items ...string) []string { return items },
	"percent": func(val, max int) int {
		if max == 0 {
			return 0
		}
		return val * 100 / max
	},
}

func unmarshalFn[T any](data []byte) T {
	var v T
	json.Unmarshal(data, &v)
	return v
}

func LoadTemplates(dir string) {
	t := template.New("").Funcs(funcMap)
	t = template.Must(t.ParseGlob(filepath.Join(dir, "*.html")))
	templates = t
}

func renderTemplate(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template %s error: %v", name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func renderPartial(w http.ResponseWriter, name string, data any) {
	if err := templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("partial %s error: %v", name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
