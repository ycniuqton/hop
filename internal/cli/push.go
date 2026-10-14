package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/complete"
	"github.com/ycniuqton/hop/internal/target"
	"github.com/ycniuqton/hop/internal/ui"
	"github.com/ycniuqton/hop/internal/workspace"
)

// PushCommand puts files on a hot shelf (or bridges a store file onto one).
type PushCommand struct {
	env *Env
	as  string
}

func (c *PushCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "push <file>... [@device|@]", Short: "Put files on a hot shelf", GroupID: string(GroupHot),
		Example: "  hop push dump.sql            # shared shelf\n  hop push dump.sql @db-1      # db-1's shelf\n  hop push @web-1:backups/x.sql @   # store → my shelf"}
	cmd.Flags().StringVar(&c.as, "as", "", "name on the shelf (one file only)")
	cmd.ValidArgsFunction = env.completer().Hot(complete.HotFiles)
	return bindHandler(cmd, atLeast1, c)
}

func (c *PushCommand) Run(ctx context.Context, args []string) error {
	inv, err := hotArgs{env: c.env, allowRemote: true}.parse(ctx, args)
	if err != nil {
		return err
	}
	if len(inv.others) == 0 {
		return apperr.New(apperr.ExitUsage, "nothing to push").WithHint("name a file")
	}
	if c.as != "" && len(inv.others) > 1 {
		return apperr.New(apperr.ExitUsage, "--as works with one file only")
	}
	for _, t := range inv.others {
		hf, err := c.pushOne(ctx, inv, t)
		if err != nil {
			return err
		}
		c.env.Console.Printf("pushed %s → %s (%s)", hf.Name, hf.Shelf.Label(), ui.Size(hf.Size))
	}
	return nil
}

func (c *PushCommand) pushOne(ctx context.Context, inv hotInvocation, t target.Target) (workspace.HotFile, error) {
	if t.Kind == target.Remote {
		return inv.svc.Shelves.PushFromStore(ctx, inv.shelf, t, c.as)
	}
	return inv.svc.Shelves.Push(ctx, inv.shelf, t.Path, c.as)
}
