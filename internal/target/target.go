// Package target parses every positional argument into a typed Local, Device or Remote target (spec section 6).
package target

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/naming"
)

// Kind classifies an argument.
type Kind int

const (
	Local Kind = iota
	Device
	Remote
)

// Syntax holds the special characters of the @ notation.
var Syntax = struct {
	DeviceMark string
	PathSep    string
	DirSuffix  string
	ParentDir  string
	LocalHint  string
}{DeviceMark: "@", PathSep: ":", DirSuffix: "/", ParentDir: "..", LocalHint: "./"}

// Target is one parsed argument.
type Target struct {
	Kind   Kind
	Raw    string
	Device string
	Path   string
	Dir    bool
}

// Parser resolves @ and @me against the configured device.
type Parser struct {
	Self string
}

// Parse classifies and validates a single argument.
func (p Parser) Parse(arg string) (Target, error) {
	rest, isDevice := strings.CutPrefix(arg, Syntax.DeviceMark)
	if !isDevice {
		dir := strings.HasSuffix(arg, Syntax.DirSuffix) || strings.HasSuffix(arg, string(filepath.Separator))
		return Target{Kind: Local, Raw: arg, Path: arg, Dir: dir}, nil
	}
	name, rawPath, hasPath := strings.Cut(rest, Syntax.PathSep)
	device, err := p.resolveDevice(name)
	if err != nil {
		return Target{}, err
	}
	if !hasPath {
		return Target{Kind: Device, Raw: arg, Device: device}, nil
	}
	clean, err := p.cleanRemotePath(arg, rawPath)
	if err != nil {
		return Target{}, err
	}
	dir := clean == "" || strings.HasSuffix(rawPath, Syntax.DirSuffix)
	return Target{Kind: Remote, Raw: arg, Device: device, Path: clean, Dir: dir}, nil
}

func (p Parser) resolveDevice(name string) (string, error) {
	if name == "" || name == naming.SelfAlias {
		return p.Self, nil
	}
	return name, naming.Devices.Validate(name)
}

func (p Parser) cleanRemotePath(arg, raw string) (string, error) {
	trimmed := strings.TrimLeft(raw, Syntax.DirSuffix)
	if trimmed == "" {
		return "", nil
	}
	clean := path.Clean(trimmed)
	if clean == Syntax.ParentDir || strings.HasPrefix(clean, Syntax.ParentDir+Syntax.DirSuffix) {
		return "", apperr.New(apperr.ExitUsage, "path %q escapes the device folder", arg).
			WithHint("remove '..' from the path")
	}
	if clean == "." {
		return "", nil
	}
	return clean, nil
}

// ParseAll parses every argument in order.
func (p Parser) ParseAll(args []string) (List, error) {
	out := make(List, 0, len(args))
	for _, a := range args {
		t, err := p.Parse(a)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}
