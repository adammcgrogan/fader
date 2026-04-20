package handlers

import (
	"bufio"
	"bytes"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type Commit struct {
	Hash    string
	Type    string
	Scope   string
	Message string
	Date    time.Time
	DateStr string
}

func Changelog(w http.ResponseWriter, r *http.Request) {
	commits, err := loadCommits()
	if err != nil {
		http.Error(w, "could not load changelog", http.StatusInternalServerError)
		return
	}
	renderTemplate(w, "changelog.html", map[string]any{
		"Commits": commits,
	})
}

func loadCommits() ([]Commit, error) {
	out, err := exec.Command("git", "log", "--pretty=format:%H|%s|%ad", "--date=short").Output()
	if err != nil {
		return nil, err
	}

	var commits []Commit
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		hash, subject, dateStr := parts[0], parts[1], parts[2]

		t, _ := time.Parse("2006-01-02", dateStr)

		ctype, scope, msg := parseConventional(subject)

		commits = append(commits, Commit{
			Hash:    hash[:7],
			Type:    ctype,
			Scope:   scope,
			Message: msg,
			Date:    t,
			DateStr: t.Format("2 Jan 2006"),
		})
	}
	return commits, scanner.Err()
}

func parseConventional(subject string) (ctype, scope, msg string) {
	// e.g. "feat(auth): add login" or "fix: correct redirect"
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
