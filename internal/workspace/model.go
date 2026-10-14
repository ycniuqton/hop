package workspace

import (
	"crypto/rand"
	"fmt"
	"time"
)

// Info is .hop/workspace.json.
type Info struct {
	Version     int       `json:"version"`
	WorkspaceID string    `json:"workspace_id"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"`
}

// Device is .hop/devices/<name>.json.
type Device struct {
	Name       string    `json:"name"`
	DeviceID   string    `json:"device_id"`
	Hostname   string    `json:"hostname"`
	OS         string    `json:"os"`
	HopVersion string    `json:"hop_version"`
	JoinedAt   time.Time `json:"joined_at"`
	LastSeen   time.Time `json:"last_seen"`
}

// Schema is the workspace.json format version written by this build.
const Schema = 1

// IDs generates random identifiers.
type IDs struct{}

// UUID returns a random RFC 4122 version 4 UUID.
func (IDs) UUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Token returns a short random hex string.
func (IDs) Token() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// Meta are the S3 user-metadata keys hop writes on transferred objects.
var Meta = struct{ From, Mode string }{From: "hop-from", Mode: "hop-mode"}
