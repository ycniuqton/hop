package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/config"
	"github.com/ycniuqton/hop/internal/naming"
	"github.com/ycniuqton/hop/internal/store"
	"github.com/ycniuqton/hop/internal/workspace"
)

// InitCommand creates a workspace and registers this device.
type InitCommand struct {
	env         *Env
	cfg         config.Config
	printPolicy bool
	providerID  string
	url         string
	provider    store.Provider
}

func (c *InitCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "init", Short: "Create a workspace in an S3 bucket and register this device", GroupID: string(GroupSetup)}
	f := cmd.Flags()
	f.StringVar(&c.providerID, "provider", "", "storage provider: "+strings.Join(store.Providers.IDs(), ", ")+" (asked when omitted)")
	f.StringVar(&c.url, "url", "", "bucket URL, e.g. https://my-bucket.hcm.ss.bfcplatform.vn (sets provider, endpoint and bucket)")
	f.StringVar(&c.cfg.Bucket, "bucket", "", "bucket name")
	f.StringVar(&c.cfg.Region, "region", "", "AWS region (default $AWS_REGION; fixed by other providers)")
	f.StringVar(&c.cfg.Prefix, "prefix", "", "key prefix inside the bucket (optional)")
	f.StringVar(&c.cfg.Device, "name", "", "name for this device (default: hostname)")
	f.StringVar(&c.cfg.Endpoint, "endpoint", "", "S3-compatible endpoint URL (provider 's3', e.g. MinIO)")
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
	c.env.Console.Successf("Created hop workspace on %s in %s/%s as %q.", c.provider.Title, c.cfg.Bucket, workspace.NewLayout(c.cfg.Prefix).String(), c.cfg.Device)
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
	if err := c.chooseProvider(); err != nil {
		return err
	}
	if err := c.collectLocation(); err != nil {
		return err
	}
	if err := c.collectKeys(); err != nil {
		return err
	}
	return c.collectName()
}

// chooseProvider takes the provider from flags, else asks with Bizfly Cloud as the default.
func (c *InitCommand) chooseProvider() error {
	p, err := c.providerFromFlags()
	if err != nil || p.ID != "" {
		c.provider = p
		return err
	}
	list := store.Providers.List()
	titles := make([]string, len(list))
	for i, it := range list {
		titles[i] = it.Title
	}
	i, err := c.env.Console.Choose("Storage provider:", titles, 0)
	c.provider = list[i]
	return err
}

func (c *InitCommand) providerFromFlags() (store.Provider, error) {
	switch {
	case c.url != "":
		loc, err := store.Providers.ParseURL(c.url)
		c.cfg.Bucket, c.cfg.Endpoint = loc.Bucket, loc.Endpoint
		return loc.Provider, err
	case c.providerID != "":
		return store.Providers.Lookup(c.providerID)
	case c.cfg.Endpoint != "" || !c.env.Console.CanPrompt():
		return store.Providers.FromEndpoint(c.cfg.Endpoint), nil
	}
	return store.Provider{}, nil
}

// collectLocation asks only what the provider does not fix: endpoint, bucket (name or URL), region.
func (c *InitCommand) collectLocation() error {
	if c.cfg.Endpoint == "" && c.provider.AsksEndpoint {
		ep, err := c.env.Console.Ask("Endpoint URL", "", "--endpoint")
		if err != nil {
			return err
		}
		c.cfg.Endpoint = store.Providers.Endpoint(ep)
	}
	if c.cfg.Endpoint == "" {
		c.cfg.Endpoint = c.provider.Endpoint
	}
	if c.cfg.Bucket == "" {
		answer, err := c.env.Console.Ask(c.provider.BucketPrompt, "", "--bucket")
		if err != nil {
			return err
		}
		loc, err := store.Providers.Locate(c.provider, c.cfg.Endpoint, answer)
		if err != nil {
			return err
		}
		c.provider, c.cfg.Endpoint, c.cfg.Bucket = loc.Provider, loc.Endpoint, loc.Bucket
	}
	return c.collectRegion()
}

func (c *InitCommand) collectRegion() error {
	if c.cfg.Region == "" {
		c.cfg.Region = c.provider.Region
	}
	if c.cfg.Region != "" {
		return nil
	}
	get := c.env.Getenv
	region, err := c.env.Console.Ask("AWS region", c.firstOf(get(EnvVars.AWSRegion), get(EnvVars.AWSDefaultRegion)), "--region")
	c.cfg.Region = region
	return err
}

func (c *InitCommand) collectKeys() error {
	con, get, keys := c.env.Console, c.env.Getenv, c.provider.Keys
	steps := []struct {
		dst          *string
		label, def   string
		flag         string
		secret, skip bool
	}{
		{&c.cfg.AccessKeyID, keys.Access, get(EnvVars.AWSKey), EnvVars.AWSKey, false, false},
		{&c.cfg.SecretAccessKey, keys.Secret, get(EnvVars.AWSSecret), EnvVars.AWSSecret, true, false},
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
	return nil
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
