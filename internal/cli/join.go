package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/config"
	"github.com/ycniuqton/hop/internal/joincode"
	"github.com/ycniuqton/hop/internal/workspace"
)

// JoinCommand registers this machine in an existing workspace.
type JoinCommand struct {
	env   *Env
	name  string
	force bool
}

func (c *JoinCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "join <code>", Short: "Join a workspace with a code from 'hop invite'", GroupID: string(GroupSetup),
		Example: "  hop join hop1:aG9w... --name web-1\n  hop join --name web-1 < code.txt"}
	cmd.Flags().StringVar(&c.name, "name", "", "name for this device (default: hostname)")
	cmd.Flags().BoolVar(&c.force, "force", false, "replace an existing local config")
	return bindHandler(cmd, upToOne, c)
}

func (c *JoinCommand) Run(ctx context.Context, args []string) error {
	cfgStore := c.env.ConfigStore()
	if cfgStore.Exists() && !c.force {
		return apperr.New(apperr.ExitConflict, "this machine already has a hop config (%s)", cfgStore.Path).
			WithHint("use --force to replace it")
	}
	p, err := c.readCode(args)
	if err != nil {
		return err
	}
	cfg := &config.Config{Bucket: p.Bucket, Region: p.Region, Prefix: p.Prefix, Endpoint: p.Endpoint,
		AccessKeyID: p.AccessKeyID, SecretAccessKey: p.SecretAccessKey, WorkspaceID: p.WorkspaceID}
	ws, err := c.env.OpenWorkspace(ctx, cfg)
	if err != nil {
		return err
	}
	setup := workspace.NewSetup(ws)
	if err := (workspaceCheck{setup: setup}).Matches(ctx, p.WorkspaceID); err != nil {
		return err
	}
	if cfg.Device, err = c.register(ctx, setup); err != nil {
		return err
	}
	if err := cfgStore.Save(cfg); err != nil {
		return err
	}
	count := setup.DeviceCount(ctx)
	completionOffer{env: c.env}.Offer()
	c.env.Console.Successf("Joined workspace as %q (%d devices).", cfg.Device, count)
	return nil
}

func (c *JoinCommand) readCode(args []string) (joincode.Payload, error) {
	con := c.env.Console
	var raw string
	switch {
	case len(args) == 1:
		raw = args[0]
	case !con.CanPrompt():
		data, err := io.ReadAll(con.In)
		if err != nil {
			return joincode.Payload{}, err
		}
		raw = strings.TrimSpace(string(data))
	default:
		v, err := con.AskSecret("Join code", "", "<code>")
		if err != nil {
			return joincode.Payload{}, err
		}
		raw = v
	}
	return joincode.Current.Decode(raw)
}

func (c *JoinCommand) register(ctx context.Context, setup *workspace.SetupManager) (string, error) {
	host, _ := c.env.Hostname()
	for {
		name, err := deviceNamer{env: c.env}.Pick(c.name)
		if err != nil {
			return "", err
		}
		_, err = setup.Register(ctx, name, host)
		if !errors.Is(err, workspace.ErrNameTaken) || c.name != "" || !c.env.Console.CanPrompt() {
			return name, err
		}
		fmt.Fprintf(c.env.Console.Err, "error: %s\n", err)
	}
}

// workspaceCheck verifies credentials and that the bucket holds the expected workspace.
type workspaceCheck struct {
	setup *workspace.SetupManager
}

func (w workspaceCheck) Matches(ctx context.Context, wantID string) error {
	if err := w.setup.Verify(ctx); err != nil {
		return err
	}
	info, err := w.setup.Info(ctx)
	if err != nil {
		return err
	}
	if info.WorkspaceID != wantID {
		return apperr.New(apperr.ExitConflict, "the bucket holds a different hop workspace than this code").
			WithHint("ask for a fresh code with 'hop invite'")
	}
	return nil
}
