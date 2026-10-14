package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ycniuqton/hop/internal/apperr"
)

// jsonDoc parses exactly one JSON object; trailing output would make Decode's second call succeed.
func jsonDoc(t *testing.T, out string) map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(out))
	var doc map[string]any
	require.NoError(t, dec.Decode(&doc), out)
	assert.False(t, dec.More(), "exactly one JSON document: %s", out)
	return doc
}

func TestInitOneLineWithHopEnv(t *testing.T) {
	m := withoutAWSEnv(newMemoryHarness(t).machine("web-1"))
	m.vars[EnvVars.HopKey], m.vars[EnvVars.HopSecret] = "HOPKEY", "hop-secret-value"
	m.vars[EnvVars.AWSKey], m.vars[EnvVars.AWSSecret] = "AWSKEY", "aws-secret-value"
	out := m.ok("init", "--url", "https://hop-cli.hcm.ss.bfcplatform.vn", "--name", "web-1", "--json")
	doc := jsonDoc(t, out)
	assert.Equal(t, "init", doc["action"])
	assert.Equal(t, "bizfly", doc["provider"])
	assert.Equal(t, "hop-cli", doc["bucket"])
	assert.NotContains(t, out, "secret-value")
	cfg, err := (&Env{Getenv: func(k string) string { return m.vars[k] }}).ConfigStore().Load()
	require.NoError(t, err)
	assert.Equal(t, "HOPKEY", cfg.AccessKeyID, "HOP_ACCESS_KEY wins over AWS_ACCESS_KEY_ID")
	assert.Equal(t, "hop-secret-value", cfg.SecretAccessKey)
}

func TestInitSecretFromStdin(t *testing.T) {
	m := withoutAWSEnv(newMemoryHarness(t).machine("web-1"))
	m.vars[EnvVars.HopKey] = "HOPKEY"
	r := m.run("piped-secret\n", "init", "--provider", "bizfly", "--bucket", "hop-cli", "--name", "web-1", "--secret-stdin")
	require.Equal(t, apperr.ExitOK, r.code, r.err)
	assert.NotContains(t, r.out+r.err, "piped-secret")
	cfg, err := (&Env{Getenv: func(k string) string { return m.vars[k] }}).ConfigStore().Load()
	require.NoError(t, err)
	assert.Equal(t, "piped-secret", cfg.SecretAccessKey)

	empty := withoutAWSEnv(newMemoryHarness(t).machine("x"))
	empty.vars[EnvVars.HopKey] = "HOPKEY"
	assert.Contains(t, empty.fails(apperr.ExitUsage, "init", "--url", "https://b.hcm.ss.bfcplatform.vn", "--name", "x", "--secret-stdin"),
		"--secret-stdin: no secret key on stdin")
}

func TestInitMissingKeysNameTheFix(t *testing.T) {
	m := withoutAWSEnv(newMemoryHarness(t).machine("x"))
	assert.Equal(t, "error: missing Access key — pass HOP_ACCESS_KEY (env)",
		m.fails(apperr.ExitUsage, "init", "--url", "https://b.hcm.ss.bfcplatform.vn", "--name", "x"))
	m.vars[EnvVars.HopKey] = "HOPKEY"
	assert.Equal(t, "error: missing Secret key — pass HOP_SECRET_KEY (env) or --secret-stdin",
		m.fails(apperr.ExitUsage, "init", "--url", "https://b.hcm.ss.bfcplatform.vn", "--name", "x"))
}

func TestJSONErrors(t *testing.T) {
	a, _ := newMemoryHarness(t).pair()
	r := a.run("", "pull", "nope.txt", "--json")
	assert.Equal(t, apperr.ExitNotFound, r.code)
	assert.Empty(t, r.out, "stdout stays empty on failure")
	assert.Equal(t, map[string]any{"error": `"nope.txt" not on shared shelf`, "hint": "", "code": float64(3), "kind": "not_found"}, jsonDoc(t, r.err))

	r = a.run("", "push", a.file("x", "x"), "@web1", "--json")
	assert.Equal(t, map[string]any{"error": `unknown device "web1"`, "hint": "did you mean web-1?", "code": float64(3), "kind": "not_found"}, jsonDoc(t, r.err))

	r = a.run("", "ls", "--bogus", "--json")
	assert.Equal(t, "usage", jsonDoc(t, r.err)["kind"])
}

// TestJSONActionResults checks every action command prints one JSON object naming its action.
func TestJSONActionResults(t *testing.T) {
	a, b := newMemoryHarness(t).pair()
	src, other := a.file("db.sql", "data"), a.file("two.txt", "2")
	steps := []struct {
		m      *machine
		action string
		args   []string
		check  func(map[string]any)
	}{
		{a, "push", []string{"push", src, other, "@web-1"}, func(d map[string]any) {
			assert.Equal(t, "web-1", d["shelf"])
			assert.Len(t, d["files"], 2)
		}},
		{b, "pull", []string{"pull", "db.sql", "@", b.path("db.sql")}, func(d map[string]any) { assert.Equal(t, b.path("db.sql"), d["path"]) }},
		{b, "keep", []string{"keep", "two.txt", "@", "@:kept/"}, func(d map[string]any) { assert.Equal(t, "@web-1:kept/two.txt", d["to"]) }},
		{b, "hot_rm", []string{"hot", "rm", "two.txt", "@"}, func(d map[string]any) { assert.Equal(t, []any{"two.txt"}, d["deleted"]) }},
		{a, "cp", []string{"cp", src, "@web-1:backups/"}, func(d map[string]any) { assert.EqualValues(t, 1, d["files"]) }},
		{a, "mv", []string{"mv", "@web-1:backups/db.sql", "@web-1:old/db.sql"}, func(d map[string]any) { assert.EqualValues(t, 1, d["files"]) }},
		{a, "rm", []string{"rm", "-r", "@web-1:old/"}, func(d map[string]any) { assert.EqualValues(t, 1, d["deleted"]) }},
		{a, "hot_clear", []string{"hot", "clear", "-y"}, func(d map[string]any) { assert.EqualValues(t, 0, d["deleted"]) }},
		{a, "rename", []string{"rename", "web-1", "web-2"}, func(d map[string]any) { assert.Equal(t, "web-2", d["to"]) }},
		{a, "remove", []string{"remove", "web-2", "-y"}, func(d map[string]any) { assert.Contains(t, d["warning"], "still holds working S3 keys") }},
	}
	for _, s := range steps {
		r := s.m.run("", append(s.args, "--json")...)
		require.Equal(t, apperr.ExitOK, r.code, "%v: %s", s.args, r.err)
		doc := jsonDoc(t, r.out)
		assert.Equal(t, s.action, doc["action"], "%v", s.args)
		s.check(doc)
	}
}

func TestJSONJoinAndCompletionInstall(t *testing.T) {
	h := newMemoryHarness(t)
	a, b := h.machine("laptop"), h.machine("web-1")
	a.ok("init", "--bucket", "my-hop", "--name", "laptop")
	doc := jsonDoc(t, b.ok("join", strings.TrimSpace(a.ok("invite", "--code")), "--name", "web-1", "--json"))
	assert.Equal(t, "join", doc["action"])
	assert.EqualValues(t, 2, doc["devices"])
	assert.NotContains(t, b.ok("whoami", "--json"), "test-secret-value")

	b.vars[EnvVars.Shell] = "/bin/bash"
	doc = jsonDoc(t, b.ok("completion", "install", "--json"))
	assert.Equal(t, "completion_install", doc["action"])
	assert.Equal(t, b.path(".local/share/bash-completion/completions/hop"), doc["path"])
}

func TestTextOutputUnchangedWithoutJSON(t *testing.T) {
	a, _ := newMemoryHarness(t).pair()
	assert.Equal(t, "ok: pushed x → @web-1's shelf (1 B)\n", a.ok("push", a.file("x", "x"), "@web-1"))
	assert.Equal(t, `error: "nope" not on shared shelf`, a.fails(apperr.ExitNotFound, "pull", "nope"))
}

// TestSkillRecipesWork runs the skill's recipes the way an agent would, all with --json.
func TestSkillRecipesWork(t *testing.T) {
	h := newMemoryHarness(t)
	fresh := h.machine("fresh")
	r := fresh.run("", "whoami", "--json")
	assert.Equal(t, apperr.ExitGeneral, r.code)
	assert.Equal(t, "general", jsonDoc(t, r.err)["kind"])

	a, b := h.pair()
	db := h.machine("db-1")
	r = db.run(strings.TrimSpace(a.ok("invite", "--code"))+"\n", "join", "--name", "db-1", "--json")
	require.Equal(t, apperr.ExitOK, r.code, r.err)

	a.ok("push", a.file("dump.sql", "x"), "@web-1", "--json")
	assert.Contains(t, b.ok("hot", "@", "--json"), `"name": "dump.sql"`)
	jsonDoc(t, b.ok("pull", "dump.sql", "@", b.path("dump.sql"), "--rm", "--json"))
	a.file("logs/app.log", "log")
	jsonDoc(t, a.ok("cp", "-r", a.path("logs"), "@:logs/", "--json"))
	assert.Contains(t, a.ok("ls", "-R", "@:logs/", "--json"), "app.log")
	b.ok("cp", b.file("db.sql", "d"), "@:backups/db.sql")
	jsonDoc(t, a.ok("cp", "@web-1:backups/db.sql", a.path("db.sql"), "--json"))
	jsonDoc(t, a.ok("cp", "@web-1:backups/db.sql", "@db-1:restore/", "--json"))
	a.ok("push", a.path("dump.sql"))
	assert.Equal(t, "@laptop:backups/dump.sql", jsonDoc(t, a.ok("keep", "dump.sql", "@:backups/", "--json"))["to"])
	jsonDoc(t, a.ok("push", "@web-1:backups/db.sql", "@db-1", "--json"))
	assert.Contains(t, db.ok("hot", "@", "--json"), `"name": "db.sql"`)
	assert.Contains(t, a.ok("status", "--json"), `"store"`)
}
