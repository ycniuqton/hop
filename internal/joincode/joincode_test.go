package joincode

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
)

func sample() Payload {
	return Payload{Bucket: "my-hop", Region: "ap-southeast-1", AccessKeyID: "AKIA1",
		SecretAccessKey: "a+b/c=d", WorkspaceID: "7f2c9a1e"}
}

func TestRoundTrip(t *testing.T) {
	code := Current.Encode(sample())
	assert.True(t, strings.HasPrefix(code, "hop1:"))
	assert.NotContains(t, code[len("hop1:"):], "+")
	assert.NotContains(t, code, "/")
	assert.NotContains(t, code, "=")

	for _, pasted := range []string{code, "  " + code + "\n", "hop join " + code, "hop join   " + code} {
		got, err := Current.Decode(pasted)
		require.NoError(t, err, pasted)
		want := sample()
		want.Version = 1
		assert.Equal(t, want, got)
	}
}

func TestDecodeRejects(t *testing.T) {
	missingWorkspace := sample()
	missingWorkspace.WorkspaceID = ""
	cases := map[string]string{
		"aG9w":     "missing \"hop1:\" prefix",
		"hop2:abc": "\"hop2\" is not supported",
		"hop1:!!!": "not valid base64url",
		"hop1:" + encoding.EncodeToString([]byte("[")):       "not valid JSON",
		"hop1:" + encoding.EncodeToString([]byte(`{"v":7}`)): "unsupported payload version",
		Current.Encode(missingWorkspace):                     "missing workspace id",
		"":                                                   "missing",
	}
	for in, want := range cases {
		_, err := Current.Decode(in)
		assert.ErrorContains(t, err, want, in)
		assert.Equal(t, apperr.ExitUsage, apperr.CodeOf(err), in)
	}
}
