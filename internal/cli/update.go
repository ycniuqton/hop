package cli

import (
	"context"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/selfinstall"
	"github.com/ycniuqton/hop/internal/ui"
)

// UpdateCommand replaces this binary with the latest (or a pinned) release.
type UpdateCommand struct {
	env     *Env
	check   bool
	version string
	force   bool
}

func (c *UpdateCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "update", Short: "Update hop to the latest release", GroupID: string(GroupSetup)}
	f := cmd.Flags()
	f.BoolVar(&c.check, "check", false, "only report whether an update is available")
	f.StringVar(&c.version, "version", "", "install this release instead of the latest (e.g. v0.2.0)")
	f.BoolVar(&c.force, "force", false, "reinstall even when this build is current or newer")
	return bindHandler(cmd, noArgs, c)
}

func (c *UpdateCommand) Run(ctx context.Context, _ []string) error {
	rel := selfinstall.NewReleases(c.env.Getenv(EnvVars.ReleaseBase), c.env.Getenv(EnvVars.Repo))
	tag, err := c.target(ctx, rel)
	if err != nil {
		return err
	}
	current := strings.TrimPrefix(c.env.Version, "v")
	order := selfinstall.ParseVersion(current).Compare(selfinstall.ParseVersion(tag))
	data := map[string]any{"current": current, "latest": strings.TrimPrefix(tag, "v"), "update_available": order < 0}
	switch {
	case c.check:
		return c.report(order, data)
	case c.force || order < 0 || c.version != "":
		return c.install(ctx, rel, tag, data)
	}
	return c.report(order, data)
}

// target is the pinned --version (with a "v" prefix) or the newest release tag.
func (c *UpdateCommand) target(ctx context.Context, rel selfinstall.Releases) (string, error) {
	if c.version == "" {
		return rel.Latest(ctx)
	}
	return "v" + strings.TrimPrefix(c.version, "v"), nil
}

func (c *UpdateCommand) report(order int, data map[string]any) error {
	out := ui.Outcome{Action: ui.Actions.UpdateCheck, Data: data}
	switch {
	case order < 0:
		return c.env.Console.Done(out, "update available: %s → %s (run 'hop update')", data["current"], data["latest"])
	case order > 0:
		return c.env.Console.Done(out, "hop %s is newer than the latest release %s (use --force to install %s)",
			data["current"], data["latest"], data["latest"])
	}
	return c.env.Console.Done(out, "hop %s is up to date", data["current"])
}

func (c *UpdateCommand) install(ctx context.Context, rel selfinstall.Releases, tag string, data map[string]any) error {
	bin, err := selfinstall.Locate(c.env.Executable)
	if err != nil {
		return err
	}
	payload, err := rel.Fetch(ctx, tag, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	if err := bin.Replace(payload); err != nil {
		return err
	}
	data["path"] = bin.Path
	if err := c.env.Console.Done(ui.Outcome{Action: ui.Actions.Update, Data: data},
		"updated hop %s → %s (%s)", data["current"], data["latest"], bin.Path); err != nil {
		return err
	}
	c.env.Console.Printf("If you use the agent skill, refresh it with 'hop skill install'.")
	return nil
}
