package stalk

import (
	"regexp"
	"strings"
)

// Twitch logins are 1–25 chars of a-z, 0-9, underscore. Display names match the
// same character set (we lowercase before storing/matching).
var loginRe = regexp.MustCompile(`^[a-z0-9_]{1,25}$`)

// NormalizeTarget strips a leading @, lowercases, and checks the result looks
// like a Twitch login/display name. ok is false for empty/junk input.
func NormalizeTarget(raw string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimPrefix(s, "@")
	s = strings.TrimSpace(s)
	if !loginRe.MatchString(s) {
		return "", false
	}
	return s, true
}
