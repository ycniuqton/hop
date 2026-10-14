//go:build integration

// Package integration runs the real hop binary against MinIO in Docker (spec section 11).
package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
)

// MinIO describes the throwaway server; the image must support conditional writes (If-None-Match).
var MinIO = struct{ Image, User, Password, Bucket, Region string }{
	Image: "minio/minio:RELEASE.2025-04-22T22-12-26Z", User: "hopadmin", Password: "hopadmin-secret", Bucket: "my-hop", Region: "us-east-1",
}

// Binary locates the hop executable built by `make build`; HOP_BIN overrides it.
var Binary = struct{ EnvOverride, Default string }{EnvOverride: "HOP_BIN", Default: "../bin/hop"}

// Exit codes the script asserts on (spec section 10).
const (
	exitOK       = 0
	exitConflict = 4
)

// device is one simulated machine: its own HOME, sharing the MinIO bucket.
type device struct {
	t        *testing.T
	bin      string
	home     string
	endpoint string
}

func (d device) run(stdin string, args ...string) (string, int) {
	cmd := exec.Command(d.bin, args...)
	cmd.Env = append(os.Environ(), "HOME="+d.home, "XDG_CONFIG_HOME=", "XDG_CACHE_HOME=", "HOP_CONFIG=", "SHELL=",
		"AWS_ACCESS_KEY_ID="+MinIO.User, "AWS_SECRET_ACCESS_KEY="+MinIO.Password, "AWS_REGION="+MinIO.Region)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return errb.String(), exit.ExitCode()
	}
	require.NoError(d.t, err)
	return out.String(), exitOK
}

func (d device) ok(args ...string) string {
	d.t.Helper()
	out, code := d.run("", args...)
	require.Equal(d.t, exitOK, code, "hop %v: %s", args, out)
	return out
}

func (d device) path(rel string) string { return filepath.Join(d.home, rel) }

func (d device) write(rel string, body []byte) string {
	p := d.path(rel)
	require.NoError(d.t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(d.t, os.WriteFile(p, body, 0o644))
	return p
}

// world starts MinIO, creates the bucket and returns two unconfigured devices.
type world struct {
	t        *testing.T
	endpoint string
	bin      string
}

func newWorld(t *testing.T) world {
	testcontainers.SkipIfProviderIsNotHealthy(t)
	ctx := context.Background()
	c, err := minio.Run(ctx, MinIO.Image, minio.WithUsername(MinIO.User), minio.WithPassword(MinIO.Password))
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	conn, err := c.ConnectionString(ctx)
	require.NoError(t, err)
	w := world{t: t, endpoint: "http://" + conn, bin: Binary.Default}
	if p := os.Getenv(Binary.EnvOverride); p != "" {
		w.bin = p
	}
	w.bin, err = filepath.Abs(w.bin)
	require.NoError(t, err)
	w.createBucket(ctx)
	return w
}

func (w world) createBucket(ctx context.Context) {
	client := s3.New(s3.Options{
		Region: MinIO.Region, BaseEndpoint: aws.String(w.endpoint), UsePathStyle: true,
		Credentials: credentials.NewStaticCredentialsProvider(MinIO.User, MinIO.Password, ""),
	})
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(MinIO.Bucket)})
	require.NoError(w.t, err)
}

func (w world) device() device {
	return device{t: w.t, bin: w.bin, home: w.t.TempDir(), endpoint: w.endpoint}
}

// TestScript is spec section 11: init, invite, join, hot round trip, store round trip with a 50 MB file.
func TestScript(t *testing.T) {
	w := newWorld(t)
	a, b, dup := w.device(), w.device(), w.device()
	a.ok("init", "--bucket", MinIO.Bucket, "--name", "laptop", "--endpoint", w.endpoint, "--region", MinIO.Region)
	code := strings.TrimSpace(a.ok("invite", "--code"))
	b.ok("join", "--name", "web-1", code)
	msg, exit := dup.run("", "join", "--name", "web-1", code)
	assert.Equal(t, exitConflict, exit)
	assert.Contains(t, msg, "already used")

	a.ok("push", a.write("x.bin", []byte("hello over minio")), "@web-1")
	b.ok("pull", "x.bin", "@", b.path("x.bin"))
	got, _ := os.ReadFile(b.path("x.bin"))
	assert.Equal(t, "hello over minio", string(got))

	big := make([]byte, 50<<20)
	_, _ = rand.Read(big)
	b.write("folder/big.bin", big)
	b.write("folder/notes/n.txt", []byte("n"))
	b.ok("cp", "-r", b.path("folder"), "@:")
	assert.Equal(t, "folder/big.bin\nfolder/notes/n.txt\n", a.ok("ls", "-R", "@web-1:"))

	a.ok("cp", "@web-1:folder/big.bin", "@laptop:copy/")
	a.ok("cp", "@:copy/big.bin", a.path("big.bin"))
	back, _ := os.ReadFile(a.path("big.bin"))
	assert.True(t, bytes.Equal(big, back), "50 MB file round-trips byte for byte")

	a.ok("rm", "-r", "-y", "@web-1:folder/")
	assert.Empty(t, a.ok("ls", "@web-1:"))
}
