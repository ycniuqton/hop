// Package naming holds the rules for device names and hot-shelf file names.
package naming

import (
	"regexp"
	"strings"

	"github.com/ycniuqton/hop/internal/apperr"
)

// DeviceRules describes what a device name may look like.
type DeviceRules struct {
	pattern  *regexp.Regexp
	reserved map[string]bool
	maxLen   int
	fallback string
}

// Devices is the device naming policy from spec section 3.
var Devices = DeviceRules{
	pattern:  regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`),
	reserved: map[string]bool{SharedShelf: true, SelfAlias: true},
	maxLen:   32,
	fallback: "device",
}

const (
	// SharedShelf is the reserved shelf name holding the global hot list.
	SharedShelf = "_shared"
	// SelfAlias is the reserved spelling of "this device" in @ targets.
	SelfAlias = "me"
)

// Validate rejects malformed or reserved device names.
func (r DeviceRules) Validate(name string) error {
	if r.reserved[name] {
		return apperr.New(apperr.ExitUsage, "device name %q is reserved", name)
	}
	if !r.pattern.MatchString(name) {
		return apperr.New(apperr.ExitUsage, "invalid device name %q", name).
			WithHint("use lowercase letters, digits and '-', starting with a letter or digit, max %d characters", r.maxLen)
	}
	return nil
}

// FromHostname turns a hostname into a valid default device name.
func (r DeviceRules) FromHostname(host string) string {
	short, _, _ := strings.Cut(strings.ToLower(host), ".")
	var b strings.Builder
	for _, ch := range short {
		if b.Len() >= r.maxLen {
			break
		}
		b.WriteRune(r.sanitizeRune(ch))
	}
	name := strings.TrimLeft(b.String(), "-")
	if r.Validate(name) != nil {
		return r.fallback
	}
	return name
}

func (DeviceRules) sanitizeRune(ch rune) rune {
	if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
		return ch
	}
	return '-'
}

// HotNames is the naming policy for flat hot-shelf entries.
var HotNames = hotNameRules{}

type hotNameRules struct{}

// Validate rejects hot names that look like paths.
func (hotNameRules) Validate(name string) error {
	if name == "" || name == "." || name == ".." {
		return apperr.New(apperr.ExitUsage, "invalid hot name %q", name)
	}
	if strings.Contains(name, "/") {
		return apperr.New(apperr.ExitUsage, "hot names are flat").WithHint("use 'hop cp' for paths")
	}
	return nil
}
