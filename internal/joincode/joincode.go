// Package joincode encodes and decodes the `hop1:<base64url(json)>` invite codes.
package joincode

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/ycniuqton/hop/internal/apperr"
)

// Payload is everything a new device needs to join a workspace.
type Payload struct {
	Version         int    `json:"v"`
	Bucket          string `json:"b"`
	Region          string `json:"r"`
	Prefix          string `json:"p"`
	Endpoint        string `json:"e"`
	AccessKeyID     string `json:"k"`
	SecretAccessKey string `json:"s"`
	WorkspaceID     string `json:"w"`
}

// Format describes the current join code wire format.
type Format struct {
	Prefix        string
	Version       int
	PastedCommand string
	anyVersion    *regexp.Regexp
}

// Current is the join code format written and accepted by this build.
var Current = Format{
	Prefix:        "hop1:",
	Version:       1,
	PastedCommand: "hop join",
	anyVersion:    regexp.MustCompile(`^hop\d+:`),
}

var encoding = base64.RawURLEncoding

// Encode produces a shell-safe join code.
func (f Format) Encode(p Payload) string {
	p.Version = f.Version
	data, _ := json.Marshal(p)
	return f.Prefix + encoding.EncodeToString(data)
}

// Decode parses a code, tolerating a pasted leading "hop join".
func (f Format) Decode(raw string) (Payload, error) {
	body, err := f.stripPrefix(f.normalize(raw))
	if err != nil {
		return Payload{}, err
	}
	data, err := encoding.DecodeString(body)
	if err != nil {
		return Payload{}, f.invalid("not valid base64url")
	}
	var p Payload
	if err := json.Unmarshal(data, &p); err != nil {
		return Payload{}, f.invalid("payload is not valid JSON")
	}
	return p, f.validate(p)
}

func (f Format) normalize(raw string) string {
	s := strings.TrimSpace(raw)
	if rest, ok := strings.CutPrefix(s, f.PastedCommand); ok {
		s = strings.TrimSpace(rest)
	}
	return s
}

func (f Format) stripPrefix(s string) (string, error) {
	if body, ok := strings.CutPrefix(s, f.Prefix); ok {
		return body, nil
	}
	if found := f.anyVersion.FindString(s); found != "" {
		return "", apperr.New(apperr.ExitUsage, "join code format %q is not supported by this hop", strings.TrimSuffix(found, ":")).
			WithHint("upgrade hop on this machine")
	}
	return "", apperr.New(apperr.ExitUsage, "not a hop join code (missing %q prefix)", f.Prefix).
		WithHint("run 'hop invite' on an existing device and copy the whole line")
}

func (f Format) validate(p Payload) error {
	if p.Version != f.Version {
		return f.invalid("unsupported payload version")
	}
	required := map[string]string{"bucket": p.Bucket, "region": p.Region, "access key": p.AccessKeyID,
		"secret key": p.SecretAccessKey, "workspace id": p.WorkspaceID}
	for field, value := range required {
		if value == "" {
			return f.invalid("missing " + field)
		}
	}
	return nil
}

func (f Format) invalid(reason string) error {
	return apperr.New(apperr.ExitUsage, "invalid join code: %s", reason).
		WithHint("copy the whole code from 'hop invite'")
}
