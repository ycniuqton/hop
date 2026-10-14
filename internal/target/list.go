package target

import (
	"github.com/ycniuqton/hop/internal/apperr"
)

// List is an ordered set of parsed arguments with the command-mode rules of spec section 6.
type List []Target

// HotArgs is the shape every hot command works with: an optional shelf device plus the other arguments.
type HotArgs struct {
	Shelf  *Target
	Others List
}

// ForHot enforces rules 5 and 6: at most one Device target anywhere, no Remote unless allowed.
func (l List) ForHot(allowRemote bool) (HotArgs, error) {
	var out HotArgs
	for i := range l {
		t := l[i]
		if t.Kind == Remote && !allowRemote {
			return HotArgs{}, apperr.New(apperr.ExitUsage, "%s is a store path; hot commands take names", t.Raw).
				WithHint("use 'hop cp' for store paths")
		}
		if t.Kind != Device {
			out.Others = append(out.Others, t)
			continue
		}
		if out.Shelf != nil {
			return HotArgs{}, apperr.New(apperr.ExitUsage, "only one @device is allowed, got %s and %s", out.Shelf.Raw, t.Raw)
		}
		out.Shelf = &t
	}
	return out, nil
}

// ForStore enforces rule 7: store commands reject bare Device targets.
func (l List) ForStore() (List, error) {
	for _, t := range l {
		if t.Kind == Device {
			return nil, apperr.New(apperr.ExitUsage, "%s is a device, not a path", t.Raw).
				WithHint("did you mean %s%s (with a colon)?", t.Raw, Syntax.PathSep)
		}
	}
	return l, nil
}

// Devices returns the distinct device names referenced, for validation against the device list.
func (l List) Devices() []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range l {
		if t.Kind != Local && !seen[t.Device] {
			seen[t.Device] = true
			out = append(out, t.Device)
		}
	}
	return out
}
