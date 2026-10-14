package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/ui"
	"github.com/ycniuqton/hop/internal/workspace"
)

// DevicesCommand lists devices.
type DevicesCommand struct {
	env *Env
}

func (c *DevicesCommand) Build(env *Env) *cobra.Command {
	c.env = env
	return bindHandler(&cobra.Command{Use: "devices", Short: "List devices in the workspace", GroupID: string(GroupSetup)}, noArgs, c)
}

type deviceRow struct {
	workspace.Device
	You bool `json:"you"`
}

func (c *DevicesCommand) Run(ctx context.Context, _ []string) error {
	svc, err := c.env.Services(ctx)
	if err != nil {
		return err
	}
	devs, err := svc.Devices.List(ctx)
	if err != nil {
		return err
	}
	self := c.env.cfg.Device
	if c.env.Opts.JSON {
		rows := make([]deviceRow, len(devs))
		for i, d := range devs {
			rows[i] = deviceRow{Device: d, You: d.Name == self}
		}
		return c.env.Console.PrintJSON(rows)
	}
	rows := make([][]string, len(devs))
	for i, d := range devs {
		name := d.Name
		if d.Name == self {
			name += " (you)"
		}
		rows[i] = []string{name, ui.Ago(c.env.Now(), d.LastSeen), d.Hostname}
	}
	c.env.Console.Table([]string{"NAME", "LAST SEEN", "HOSTNAME"}, rows)
	return nil
}

// WhoamiCommand shows this device and its bucket.
type WhoamiCommand struct {
	env *Env
}

func (c *WhoamiCommand) Build(env *Env) *cobra.Command {
	c.env = env
	return bindHandler(&cobra.Command{Use: "whoami", Short: "Show this device, bucket, region and prefix", GroupID: string(GroupSetup)}, noArgs, c)
}

func (c *WhoamiCommand) Run(context.Context, []string) error {
	cfg, err := c.env.Config()
	if err != nil {
		return err
	}
	if c.env.Opts.JSON {
		return c.env.Console.PrintJSON(map[string]string{"device": cfg.Device, "bucket": cfg.Bucket, "region": cfg.Region,
			"prefix": cfg.Prefix, "endpoint": cfg.Endpoint, "workspace_id": cfg.WorkspaceID})
	}
	rows := [][]string{{"device", cfg.Device}, {"bucket", cfg.Bucket}, {"region", cfg.Region}, {"prefix", cfg.Prefix}}
	if cfg.Endpoint != "" {
		rows = append(rows, []string{"endpoint", cfg.Endpoint})
	}
	for _, r := range rows {
		c.env.Console.Printf("%-8s %s", r[0], r[1])
	}
	return nil
}
