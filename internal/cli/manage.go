package cli

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/joincode"
	"github.com/ycniuqton/hop/internal/ui"
	"github.com/ycniuqton/hop/internal/workspace"
)

// RenameCommand renames a device with its shelf and store folder.
type RenameCommand struct {
	env *Env
}

func (c *RenameCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "rename <old> <new>", Short: "Rename a device with its shelf and store folder", GroupID: string(GroupSetup)}
	cmd.ValidArgsFunction = c.env.completer().DeviceNames(1)
	return bindHandler(cmd, twoArgs, c)
}

func (c *RenameCommand) Run(ctx context.Context, args []string) error {
	svc, err := c.env.Services(ctx)
	if err != nil {
		return err
	}
	res, err := svc.Devices.Rename(ctx, args[0], args[1])
	if err != nil {
		return err
	}
	if res.Self {
		c.env.cfg.Device = args[1]
		if err := c.env.ConfigStore().Save(c.env.cfg); err != nil {
			return err
		}
	}
	c.env.Console.Printf("Renamed %q to %q (moved %d hot files, %d store objects).", args[0], args[1], res.HotObjects, res.StoreObjects)
	return nil
}

// RemoveCommand deletes a device record.
type RemoveCommand struct {
	env   *Env
	purge bool
	yes   bool
}

func (c *RemoveCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "remove <device>", Short: "Remove a device from the workspace", GroupID: string(GroupSetup)}
	cmd.Flags().BoolVar(&c.purge, "purge", false, "also delete the device's shelf and store folder")
	cmd.Flags().BoolVarP(&c.yes, "yes", "y", false, "do not ask for confirmation")
	cmd.ValidArgsFunction = c.env.completer().DeviceNames(1)
	return bindHandler(cmd, oneArg, c)
}

func (c *RemoveCommand) Run(ctx context.Context, args []string) error {
	svc, err := c.env.Services(ctx)
	if err != nil {
		return err
	}
	what := "device " + args[0]
	if c.purge {
		what += " and all its files"
	}
	if !c.yes {
		if ok, err := c.env.Console.Confirm("Remove "+what+"?", false); err != nil || !ok {
			return err
		}
	}
	res, err := svc.Devices.Remove(ctx, args[0], c.purge)
	if err != nil {
		return err
	}
	c.env.Console.Printf("Removed %q (%d objects purged).", args[0], res.Purged)
	c.env.Console.Warnf("%s still holds working S3 keys. To really revoke access: create new keys, run 'hop invite' with them on one device, 'hop rotate <code>' on every remaining device, then delete the old keys in IAM.", args[0])
	return nil
}

// RotateCommand replaces this device's credentials.
type RotateCommand struct {
	env *Env
}

func (c *RotateCommand) Build(env *Env) *cobra.Command {
	c.env = env
	return bindHandler(&cobra.Command{Use: "rotate <code>", Short: "Replace this device's credentials from a new join code", GroupID: string(GroupSetup)}, oneArg, c)
}

func (c *RotateCommand) Run(ctx context.Context, args []string) error {
	cfg, err := c.env.Config()
	if err != nil {
		return err
	}
	p, err := joincode.Current.Decode(args[0])
	if err != nil {
		return err
	}
	if p.WorkspaceID != cfg.WorkspaceID || p.Bucket != cfg.Bucket || p.Prefix != cfg.Prefix {
		return apperr.New(apperr.ExitConflict, "this code is for a different workspace").WithHint("use 'hop join --force' to switch workspaces")
	}
	next := *cfg
	next.AccessKeyID, next.SecretAccessKey, next.Region, next.Endpoint = p.AccessKeyID, p.SecretAccessKey, p.Region, p.Endpoint
	ws, err := c.env.OpenWorkspace(ctx, &next)
	if err != nil {
		return err
	}
	if err := (workspaceCheck{setup: workspace.NewSetup(ws)}).Matches(ctx, cfg.WorkspaceID); err != nil {
		return err
	}
	if err := c.env.ConfigStore().Save(&next); err != nil {
		return err
	}
	c.env.Console.Printf("Rotated credentials for %q (new access key %s).", cfg.Device, next.AccessKeyID)
	return nil
}

// StatusCommand shows usage per shelf and store folder.
type StatusCommand struct {
	env *Env
}

func (c *StatusCommand) Build(env *Env) *cobra.Command {
	c.env = env
	return bindHandler(&cobra.Command{Use: "status", Short: "Show object counts and sizes per shelf and store", GroupID: string(GroupSetup)}, noArgs, c)
}

func (c *StatusCommand) Run(ctx context.Context, _ []string) error {
	svc, err := c.env.Services(ctx)
	if err != nil {
		return err
	}
	usage, err := svc.Status.Usage(ctx)
	if err != nil {
		return err
	}
	if c.env.Opts.JSON {
		return c.env.Console.PrintJSON(usage)
	}
	rows := make([][]string, len(usage))
	for i, u := range usage {
		rows[i] = []string{string(u.Area), u.Name, ui.Size(u.Bytes), strconv.Itoa(u.Objects)}
	}
	c.env.Console.Table([]string{"AREA", "NAME", "SIZE", "OBJECTS"}, rows)
	return nil
}
