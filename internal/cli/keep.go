package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/target"
)

// KeepCommand copies a hot file into the store (hot → store bridge).
type KeepCommand struct {
	env   *Env
	force bool
}

func (c *KeepCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "keep <name> [@device|@] <@dev:path>", Short: "Copy a hot file into the store", GroupID: string(GroupHot),
		Example: "  hop keep dump.sql @db-1:backups/"}
	cmd.Flags().BoolVarP(&c.force, "force", "f", false, "overwrite an existing store file")
	cmd.ValidArgsFunction = env.completer().Store(false)
	return bindHandler(cmd, atLeast2, c)
}

func (c *KeepCommand) Run(ctx context.Context, args []string) error {
	inv, err := hotArgs{env: c.env, allowRemote: true}.parse(ctx, args)
	if err != nil {
		return err
	}
	if len(inv.others) != 2 || inv.others[0].Kind != target.Local || inv.others[1].Kind != target.Remote {
		return apperr.New(apperr.ExitUsage, "usage: hop keep <name> [@device|@] <@dev:path>")
	}
	ref, err := inv.svc.Shelves.Keep(ctx, inv.shelf, inv.others[0].Path, inv.others[1], c.force)
	if err != nil {
		return err
	}
	c.env.Console.Printf("kept %s → %s", inv.others[0].Path, ref)
	return nil
}
