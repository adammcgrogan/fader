package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/adammcgrogan/fader/internal/models"
)

var templates *template.Template

var BaseDomain = "fader.bio"

var funcMap = template.FuncMap{
	"baseDomain": func() string { return BaseDomain },
	"profileURL": func(handle string) string {
		if BaseDomain == "localhost:8080" || BaseDomain == "localhost" {
			return "http://" + handle + ".localhost:8080"
		}
		return "https://" + handle + "." + BaseDomain
	},
	"unmarshalBio":        unmarshalFn[models.BioData],
	"unmarshalSocial":     unmarshalFn[models.SocialData],
	"unmarshalMusicLink":  unmarshalFn[models.MusicLinkData],
	"unmarshalGig":        unmarshalFn[models.GigData],
	"unmarshalCustomLink": unmarshalFn[models.CustomLinkData],
	"unmarshalImage":      unmarshalFn[models.ImageData],
	"unmarshalVideoLink":  unmarshalFn[models.VideoLinkData],
	"unmarshalAudioEmbed": unmarshalFn[models.AudioEmbedData],
	"unmarshalRALink":     unmarshalFn[models.RALinkData],
	"unmarshalResidency":  unmarshalFn[models.ResidencyData],
	"audioEmbedURL": func(rawURL string) template.URL {
		if rawURL == "" {
			return ""
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			return ""
		}
		host := strings.ToLower(u.Host)
		if strings.Contains(host, "soundcloud.com") {
			return template.URL("https://w.soundcloud.com/player/?url=" + url.QueryEscape(rawURL) + "&color=%23ff5500&auto_play=false&hide_related=true&show_comments=false&show_user=false&show_artwork=false")
		}
		if strings.Contains(host, "mixcloud.com") {
			path := u.Path
			if !strings.HasSuffix(path, "/") {
				path += "/"
			}
			return template.URL("https://www.mixcloud.com/widget/iframe/?hide_cover=1&feed=" + url.QueryEscape(path))
		}
		return ""
	},
	"join": strings.Join,
	"list": func(items ...string) []string { return items },
	"deref": func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	},
	"blockLabel": func(t string) string {
		labels := map[string]string{
			"social": "Social Media", "music_link": "Music", "custom_link": "Custom",
			"audio_embed": "Audio Player", "ra_link": "RA Profile", "residency": "Residency",
		}
		if l, ok := labels[t]; ok {
			return l
		}
		return t
	},
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
