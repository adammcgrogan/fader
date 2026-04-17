package validate

import (
	"errors"
	"regexp"
	"strings"
)

var reservedHandles = map[string]struct{}{
	"www": {}, "admin": {}, "api": {}, "app": {}, "mail": {}, "ftp": {},
	"support": {}, "help": {}, "blog": {}, "status": {}, "cdn": {},
	"static": {}, "assets": {}, "login": {}, "signup": {}, "billing": {},
	"dashboard": {}, "edit": {}, "analytics": {}, "me": {}, "r": {},
}

var handleRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,28}[a-z0-9]$`)

func Handle(h string) error {
	h = strings.ToLower(strings.TrimSpace(h))
	if _, reserved := reservedHandles[h]; reserved {
		return errors.New("that handle is reserved")
	}
	if !handleRe.MatchString(h) {
		return errors.New("handle must be 3–30 lowercase letters, numbers, or hyphens, and cannot start or end with a hyphen")
	}
	return nil
}
