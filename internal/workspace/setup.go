package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"strings"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/store"
)

// SetupManager creates and joins workspaces (spec section 4).
type SetupManager struct {
	ws      *Workspace
	devices *DeviceManager
}

// NewSetup works on a workspace whose device is not registered yet.
func NewSetup(ws *Workspace) *SetupManager {
	return &SetupManager{ws: ws, devices: &DeviceManager{ws: ws}}
}

// VerifyStep names one credential check.
type VerifyStep string

const (
	StepHeadBucket VerifyStep = "HeadBucket"
	StepProbePut   VerifyStep = "PutObject (probe)"
	StepProbeClean VerifyStep = "DeleteObject (probe)"
)

// Verify runs HeadBucket and a probe write/delete, naming the step that failed.
func (m *SetupManager) Verify(ctx context.Context) error {
	if err := m.ws.Storage.CheckBucket(ctx); err != nil {
		return m.stepFailed(StepHeadBucket, err)
	}
	key := m.ws.Layout.ProbeKey(IDs{}.Token())
	if err := m.ws.Storage.Put(ctx, key, strings.NewReader(""), 0, store.PutOptions{}); err != nil {
		return m.stepFailed(StepProbePut, err)
	}
	if err := m.ws.Storage.Delete(ctx, key); err != nil {
		return m.stepFailed(StepProbeClean, err)
	}
	return nil
}

func (m *SetupManager) stepFailed(step VerifyStep, err error) error {
	return apperr.New(apperr.CodeOf(err), "credential check failed at %s: %v", step, err).Wrap(err)
}

// KeysTooBroad reports whether the keys can list every bucket in the account.
func (m *SetupManager) KeysTooBroad(ctx context.Context) bool {
	return m.ws.Storage.CanListAllBuckets(ctx)
}

// ErrWorkspaceExists means the bucket already holds a workspace.
var ErrWorkspaceExists = apperr.New(apperr.ExitConflict, "bucket already has a hop workspace").
	WithHint("run 'hop invite' on an existing device and use 'hop join' instead")

// Create writes workspace.json; an existing workspace is refused.
func (m *SetupManager) Create(ctx context.Context, creator string) (Info, error) {
	_, err := m.ws.Storage.Head(ctx, m.ws.Layout.WorkspaceKey())
	if err == nil {
		return Info{}, ErrWorkspaceExists
	}
	if !errors.Is(err, store.ErrNotFound) {
		return Info{}, err
	}
	info := Info{Version: Schema, WorkspaceID: IDs{}.UUID(), CreatedAt: m.ws.Now().UTC(), CreatedBy: creator}
	err = m.ws.putJSON(ctx, m.ws.Layout.WorkspaceKey(), info, true)
	if errors.Is(err, store.ErrPreconditionFailed) {
		return Info{}, ErrWorkspaceExists
	}
	return info, err
}

// Info reads workspace.json, reporting a missing workspace clearly.
func (m *SetupManager) Info(ctx context.Context) (Info, error) {
	var info Info
	err := m.ws.getJSON(ctx, m.ws.Layout.WorkspaceKey(), &info)
	if errors.Is(err, store.ErrNotFound) {
		return info, apperr.New(apperr.ExitNotFound, "no hop workspace in this bucket").WithHint("run 'hop init' first")
	}
	return info, err
}

// Register records this machine under name.
func (m *SetupManager) Register(ctx context.Context, name, hostname string) (Device, error) {
	now := m.ws.Now().UTC()
	d := Device{
		Name: name, DeviceID: IDs{}.UUID(), Hostname: hostname, OS: runtime.GOOS + "/" + runtime.GOARCH,
		HopVersion: m.ws.Version, JoinedAt: now, LastSeen: now,
	}
	if err := m.devices.Register(ctx, d); err != nil {
		return Device{}, err
	}
	m.ws.Self = name
	return d, nil
}

// DeviceCount lists devices (refreshing the cache) and returns how many there are.
func (m *SetupManager) DeviceCount(ctx context.Context) int {
	devs, _ := m.devices.List(ctx)
	return len(devs)
}

// Policy returns the minimal IAM policy for a bucket and prefix.
func (SetupManager) Policy(bucket, prefix string) string {
	resource := "arn:aws:s3:::" + bucket + "/" + NewLayout(prefix).prefix + "*"
	doc := map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{"Effect": "Allow", "Action": []string{"s3:ListBucket"}, "Resource": "arn:aws:s3:::" + bucket},
			{"Effect": "Allow", "Action": []string{"s3:GetObject", "s3:PutObject", "s3:DeleteObject"}, "Resource": resource},
		},
	}
	out, _ := json.MarshalIndent(doc, "", "  ")
	return string(out)
}
