package store_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/store"
	"github.com/ycniuqton/hop/internal/store/storetest"
)

func TestFakeContract(t *testing.T) {
	storetest.Contract(t, func(*testing.T) store.Storage { return store.NewFake() })
}

func TestManagerSelectsBackend(t *testing.T) {
	m := store.NewManager()
	s, err := m.Open(context.Background(), store.Settings{Backend: store.BackendMemory})
	require.NoError(t, err)
	assert.IsType(t, &store.Fake{}, s)

	_, err = m.Open(context.Background(), store.Settings{Backend: "nope"})
	assert.ErrorContains(t, err, "unknown storage backend")
}

func TestManagerDefaultsToS3(t *testing.T) {
	s, err := store.NewManager().Open(context.Background(), store.Settings{Bucket: "b", Region: "us-east-1", AccessKeyID: "k", SecretAccessKey: "s"})
	require.NoError(t, err)
	assert.IsType(t, &store.S3{}, s)
}

func TestTranslateErrors(t *testing.T) {
	tr := store.ErrorTranslatorForTest("my-hop")
	cases := []struct {
		name     string
		err      error
		sentinel error
		code     apperr.ExitCode
		msg      string
	}{
		{"denied", &smithy.GenericAPIError{Code: "AccessDenied"}, store.ErrAccessDenied, apperr.ExitAuth,
			"S3 access denied on PutObject my-hop/hot/_shared/x — check the IAM policy ('hop init --print-policy')"},
		{"missing", &smithy.GenericAPIError{Code: "NoSuchKey"}, store.ErrNotFound, apperr.ExitNotFound, "not found: my-hop/hot/_shared/x"},
		{"precondition", &smithy.GenericAPIError{Code: "PreconditionFailed"}, store.ErrPreconditionFailed, apperr.ExitConflict, "already exists: my-hop/hot/_shared/x"},
		{"network", &net.OpError{Op: "dial", Err: errors.New("refused")}, store.ErrNetwork, apperr.ExitNetwork, ""},
		{"other", errors.New("weird"), nil, apperr.ExitGeneral, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tr(store.OpPut, "hot/_shared/x", fmt.Errorf("op: %w", tc.err))
			assert.Equal(t, tc.code, apperr.CodeOf(err))
			if tc.sentinel != nil {
				assert.ErrorIs(t, err, tc.sentinel)
			}
			if tc.msg != "" {
				assert.EqualError(t, err, tc.msg)
			}
		})
	}
	assert.ErrorIs(t, tr(store.OpGet, "k", context.Canceled), context.Canceled)
	assert.NoError(t, tr(store.OpGet, "k", nil))
}
