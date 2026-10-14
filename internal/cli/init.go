package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/config"
	"github.com/ycniuqton/hop/internal/naming"
	"github.com/ycniuqton/hop/internal/workspace"
)

// InitCommand creates a workspace and registers this device.
type InitCommand struct {
	env         *Env
	cfg         config.Config
	printPolicy bool
}

func (c *InitCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "init", Short: "Create a workspace in an S3 bucket and register this device", GroupID: string(GroupSetup)}
	f := cmd.Flags()
	f.StringVar(&c.cfg.Bucket, "bucket", "", "S3 bucket name")
	f.StringVar(&c.cfg.Region, "region", "", "AWS region (default $AWS_REGION)")
	f.StringVar(&c.cfg.Prefix, "prefix", "", "key prefix inside the bucket (optional)")
	f.StringVar(&c.cfg.Device, "name", "", "name for this device (default: hostname)")
	f.StringVar(&c.cfg.Endpoint, "endpoint", "", "S3-compatible endpoint URL (advanced; e.g. MinIO)")
	f.BoolVar(&c.printPolicy, "print-policy", false, "print a minimal IAM policy for the bucket and exit")
	return bindHandler(cmd, noArgs, c)
}

func (c *InitCommand) Run(ctx context.Context, _ []string) error {
	if c.printPolicy {
		return c.policy()
	}
	if store := c.env.ConfigStore(); store.Exists() {
		return apperr.New(apperr.ExitConflict, "this machine is already configured (%s)", store.Path).
			WithHint("use 'hop join --force <code>' to switch workspaces")
	}
	if err := c.collect(); err != nil {
		return err
	}
	ws, err := c.env.OpenWorkspace(ctx, &c.cfg)
	if err != nil {
		return err
	}
	if err := c.create(ctx, workspace.NewSetup(ws)); err != nil {
		return err
	}
	if err := c.env.ConfigStore().Save(&c.cfg); err != nil {
		return err
	}
	completionOffer{env: c.env}.Offer()
	c.env.Console.Successf("Created hop workspace in s3://%s/%s as %q.", c.cfg.Bucket, workspace.NewLayout(c.cfg.Prefix).String(), c.cfg.Device)
	c.env.Console.Printf("Next: run 'hop invite' to add another machine.")
	return nil
}

func (c *InitCommand) policy() error {
	bucket, err := c.env.Console.Ask("S3 bucket", c.cfg.Bucket, "--bucket")
	if err != nil {
		return err
	}
	fmt.Fprintln(c.env.Console.Out, workspace.SetupManager{}.Policy(bucket, c.cfg.Prefix))
	return nil
}

func (c *InitCommand) create(ctx context.Context, setup *workspace.SetupManager) error {
	if err := setup.Verify(ctx); err != nil {
		return err
	}
	if setup.KeysTooBroad(ctx) {
		c.env.Console.Warnf("these keys can list every bucket in the account — consider a dedicated IAM user ('hop init --print-policy')")
	}
	info, err := setup.Create(ctx, c.cfg.Device)
	if err != nil {
		return err
	}
	c.cfg.WorkspaceID = info.WorkspaceID
	host, _ := c.env.Hostname()
	_, err = setup.Register(ctx, c.cfg.Device, host)
	return err
}

func (c *InitCommand) collect() error {
	con, get := c.env.Console, c.env.Getenv
	steps := []struct {
		dst          *string
		label, def   string
		flag         string
		secret, skip bool
	}{
		{&c.cfg.Bucket, "S3 bucket", "", "--bucket", false, false},
		{&c.cfg.Region, "AWS region", c.firstOf(get(EnvVars.AWSRegion), get(EnvVars.AWSDefaultRegion)), "--region", false, false},
		{&c.cfg.AccessKeyID, "AWS access key ID", get(EnvVars.AWSKey), EnvVars.AWSKey, false, false},
		{&c.cfg.SecretAccessKey, "AWS secret access key", get(EnvVars.AWSSecret), EnvVars.AWSSecret, true, false},
		{&c.cfg.Prefix, "Key prefix (optional)", "", "--prefix", false, !con.CanPrompt()},
	}
	for _, s := range steps {
		if *s.dst != "" || s.skip {
			continue
		}
		v, err := c.ask(s.label, s.def, s.flag, s.secret)
		if err != nil {
			return err
		}
		*s.dst = v
	}
	return c.collectName()
}

func (c *InitCommand) ask(label, def, flag string, secret bool) (string, error) {
	if secret {
		return c.env.Console.AskSecret(label, def, flag)
	}
	return c.env.Console.Ask(label, def, flag)
}

func (c *InitCommand) collectName() error {
	name, err := deviceNamer{env: c.env}.Pick(c.cfg.Device)
	c.cfg.Device = name
	return err
}

func (c *InitCommand) firstOf(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// deviceNamer chooses a valid device name: the flag, else a prompt defaulting to the hostname.
type deviceNamer struct {
	env *Env
}

func (n deviceNamer) Pick(flag string) (string, error) {
	if flag != "" {
		return flag, naming.Devices.Validate(flag)
	}
	host, _ := n.env.Hostname()
	def := naming.Devices.FromHostname(host)
	for {
		name, err := n.env.Console.Ask("Device name", def, "--name")
		if err != nil {
			return "", err
		}
		err = naming.Devices.Validate(name)
		if err == nil || !n.env.Console.CanPrompt() {
			return name, err
		}
		fmt.Fprintf(n.env.Console.Err, "error: %s\n", err)
	}
}
