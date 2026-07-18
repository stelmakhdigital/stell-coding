package update

import (
	"strings"

	"golang.org/x/mod/semver"
)

// IsNewer сообщает, новее ли candidate, чем current.
func IsNewer(candidate, current string) bool {
	c := normalizeSemver(candidate)
	cur := normalizeSemver(current)
	if c == "" || cur == "" {
		return strings.TrimSpace(candidate) != strings.TrimSpace(current)
	}
	return semver.Compare(c, cur) > 0
}

func normalizeSemver(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if semver.IsValid(v) {
		return v
	}
	return ""
}
