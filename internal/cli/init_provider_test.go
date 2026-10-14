package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
)

// typing answers init's prompts the way a person at a terminal would.
func (m *machine) typing(answers ...string) result {
	return m.typingArgs(answers, "init")
}

// typingArgs runs any command with answers typed at a terminal.
func (m *machine) typingArgs(answers []string, args ...string) result {
	m.tty = true
	defer func() { m.tty = false }()
	return m.run(strings.Join(answers, "\n")+"\n", args...)
}

func (m *machine) whoami() map[string]string {
	var out map[string]string
	require.NoError(m.h.t, json.Unmarshal([]byte(m.ok("whoami", "--json")), &out))
	return out
}

func withoutAWSEnv(m *machine) *machine {
	for _, k := range []string{EnvVars.AWSKey, EnvVars.AWSSecret, EnvVars.AWSRegion} {
		delete(m.vars, k)
	}
	return m
}

func TestInitInteractiveBizfly(t *testing.T) {
	m := withoutAWSEnv(newMemoryHarness(t).machine("laptop"))
	r := m.typing("", "https://hop-cli.hcm.ss.bfcplatform.vn", "BFCKEY", "bfc-secret", "", "")
	require.Equal(t, apperr.ExitOK, r.code, r.err)
	assert.Equal(t, "Storage provider:\n  1) Bizfly Cloud\n  2) Amazon S3\n  3) Other S3-compatible\nChoose [1]: "+
		"Bucket URL or name: Access key: Secret key: Key prefix (optional): Device name [laptop]: ", r.err)
	assert.Contains(t, r.out, `ok: Created hop workspace on Bizfly Cloud in hop-cli/ as "laptop".`)
	assert.Equal(t, map[string]string{"device": "laptop", "provider": "bizfly", "bucket": "hop-cli", "region": "us-east-1",
		"prefix": "", "endpoint": "https://hcm.ss.bfcplatform.vn", "workspace_id": m.whoami()["workspace_id"]}, m.whoami())
	assert.Contains(t, m.ok("whoami"), "provider  Bizfly Cloud")
}

func TestInitInteractiveBizflyBucketName(t *testing.T) {
	m := withoutAWSEnv(newMemoryHarness(t).machine("laptop"))
	r := m.typing("1", "hop-cli", "BFCKEY", "bfc-secret", "team", "web-1")
	require.Equal(t, apperr.ExitOK, r.code, r.err)
	who := m.whoami()
	assert.Equal(t, []string{"bizfly", "hop-cli", "https://hcm.ss.bfcplatform.vn", "team", "web-1"},
		[]string{who["provider"], who["bucket"], who["endpoint"], who["prefix"], who["device"]})
}

func TestInitInteractiveAmazonS3(t *testing.T) {
	m := newMemoryHarness(t).machine("laptop")
	r := m.typing("2", "my-hop", "", "", "", "", "")
	require.Equal(t, apperr.ExitOK, r.code, r.err)
	assert.Contains(t, r.err, "S3 bucket: AWS region [us-east-1]: AWS access key ID [AKIATEST]: AWS secret access key [from environment]: ")
	assert.NotContains(t, r.err, "test-secret-value")
	who := m.whoami()
	assert.Equal(t, []string{"aws", "my-hop", "us-east-1", ""}, []string{who["provider"], who["bucket"], who["region"], who["endpoint"]})
}

func TestInitInteractiveOtherProvider(t *testing.T) {
	m := newMemoryHarness(t).machine("laptop")
	r := m.typing("9", "3", "minio.local:9000/", "my-hop", "", "", "", "")
	require.Equal(t, apperr.ExitOK, r.code, r.err)
	assert.Contains(t, r.err, "error: enter a number from 1 to 3\nChoose [1]: Endpoint URL: Bucket name: Access key [AKIATEST]")
	who := m.whoami()
	assert.Equal(t, []string{"s3", "https://minio.local:9000", "my-hop", "us-east-1"},
		[]string{who["provider"], who["endpoint"], who["bucket"], who["region"]})
}

func TestInitURLFlagNeedsNoPrompts(t *testing.T) {
	m := newMemoryHarness(t).machine("laptop")
	m.ok("init", "--url", "https://hop-cli.hcm.ss.bfcplatform.vn", "--name", "laptop")
	who := m.whoami()
	assert.Equal(t, []string{"bizfly", "hop-cli", "https://hcm.ss.bfcplatform.vn"}, []string{who["provider"], who["bucket"], who["endpoint"]})
}

func TestInitProviderFlagErrors(t *testing.T) {
	m := newMemoryHarness(t).machine("laptop")
	assert.Equal(t, "error: missing Bucket URL or name — pass --bucket", m.fails(apperr.ExitUsage, "init", "--provider", "bizfly"))
	assert.Equal(t, `error: unknown provider "gcs" — use one of: bizfly, aws, s3`, m.fails(apperr.ExitUsage, "init", "--provider", "gcs"))
	assert.Contains(t, m.fails(apperr.ExitUsage, "init", "--url", "hop-cli"), `cannot read a bucket from "hop-cli"`)
	assert.Equal(t, "error: missing Endpoint URL — pass --endpoint", m.fails(apperr.ExitUsage, "init", "--provider", "s3"))
}

func TestJoinedDeviceShowsProvider(t *testing.T) {
	h := newMemoryHarness(t)
	a, b := h.machine("laptop"), h.machine("web-1")
	a.ok("init", "--provider", "bizfly", "--bucket", "hop-cli", "--name", "laptop")
	b.ok("join", strings.TrimSpace(a.ok("invite", "--code")), "--name", "web-1")
	assert.Equal(t, "bizfly", b.whoami()["provider"])
}
