package handlers

import (
	"bufio"
	"bytes"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/adammcgrogan/fader/internal/middleware"
)

type Commit struct {
	Hash    string
	Type    string
	Scope   string
	Message string
	Time    string // e.g. "14:32"
	DateStr string // e.g. "20 Apr 2026"
}

type CommitGroup struct {
	Date    string
	Commits []Commit
}

var cachedCommitGroups []CommitGroup

func LoadChangelog() {
	groups, err := loadCommitGroups()
	if err != nil {
		log.Printf("changelog: git log unavailable (%v) — changelog will be empty", err)
		return
	}
	cachedCommitGroups = groups
}

func Changelog(w http.ResponseWriter, r *http.Request) {
	_, loggedIn := middleware.GetUserID(r)
	renderTemplate(w, "changelog.html", map[string]any{
		"Groups":   cachedCommitGroups,
		"LoggedIn": loggedIn,
	})
}

func loadCommitGroups() ([]CommitGroup, error) {
	var out []byte
	if data, err := os.ReadFile("changelog.txt"); err == nil {
		out = data
	} else {
		// fallback for local development
		var execErr error
		out, execErr = exec.Command("git", "log",
			"--pretty=format:%H|%s|%ad",
			"--date=format:%d %b %Y|%H:%M",
		).Output()
		if execErr != nil {
			return nil, execErr
		}
	}

	var groups []CommitGroup
	var current *CommitGroup

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		// format: hash|subject|date|time  (4 pipe-separated parts)
		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			continue
		}
		hash, subject, dateStr, timeStr := parts[0], parts[1], parts[2], parts[3]
		ctype, scope, msg := parseConventional(subject)

		c := Commit{
			Hash:    hash[:7],
			Type:    ctype,
			Scope:   scope,
			Message: msg,
			Time:    timeStr,
			DateStr: dateStr,
		}

		if current == nil || current.Date != dateStr {
			groups = append(groups, CommitGroup{Date: dateStr})
			current = &groups[len(groups)-1]
		}
		current.Commits = append(current.Commits, c)
	}
	return groups, scanner.Err()
}

func parseConventional(subject string) (ctype, scope, msg string) {
	colon := strings.Index(subject, ":")
	if colon < 0 {
		return "other", "", subject
	}
	prefix := subject[:colon]
	msg = strings.TrimSpace(subject[colon+1:])

	if i := strings.Index(prefix, "("); i >= 0 {
		ctype = strings.ToLower(prefix[:i])
		scope = strings.Trim(prefix[i:], "()")
	} else {
		ctype = strings.ToLower(prefix)
	}
	return
}

func commitTypeLabel(t string) string {
	switch t {
	case "feat":
		return "feature"
	case "fix":
		return "fix"
	case "perf":
		return "perf"
	case "refactor":
		return "refactor"
	case "docs":
		return "docs"
	case "style":
		return "style"
	case "chore":
		return "chore"
	default:
		return t
	}
}
