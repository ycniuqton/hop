package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/workspace"
)

const refreshCommandName = "__refresh"

// RefreshCommand is the hidden background job that rewrites stale completion cache files.
type RefreshCommand struct {
	env *Env
}

func (c *RefreshCommand) Build(env *Env) *cobra.Command {
	c.env = env
	return bindHandler(&cobra.Command{Use: refreshCommandName + " <devices|shelf> [name]", Hidden: true}, ArgRule{Min: 1, Max: 2}, c)
}

func (c *RefreshCommand) Run(ctx context.Context, args []string) error {
	svc, err := c.env.Services(ctx)
	if err != nil {
		return nil
	}
	switch RefreshTarget(args[0]) {
	case RefreshDevices:
		_, _ = svc.Devices.List(ctx)
	case RefreshShelf:
		if len(args) == 2 {
			_, _ = svc.Shelves.List(ctx, workspace.Shelf(args[1]), false)
		}
	}
	return nil
}
