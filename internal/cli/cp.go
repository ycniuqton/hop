package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/ui"
	"github.com/ycniuqton/hop/internal/workspace"
)

// CopyCommand copies to, from or between device stores.
type CopyCommand struct {
	env *Env
	opt workspace.CopyOptions
}

func (c *CopyCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "cp [-r] [-f] <src>... <dst>", Short: "Copy files to, from or between device stores", GroupID: string(GroupStore),
		Example: "  hop cp db.sql @:backups/\n  hop cp -r ./logs @web-1:logs/\n  hop cp @web-1:backups/2026-10.sql .\n  hop cp @web-1:backups/2026-10.sql @db-1:restore/"}
	cmd.Flags().BoolVarP(&c.opt.Recursive, "recursive", "r", false, "copy folders")
	cmd.Flags().BoolVarP(&c.opt.Force, "force", "f", false, "overwrite existing files")
	cmd.ValidArgsFunction = env.completer().Store(true)
	return bindHandler(cmd, atLeast2, c)
}

func (c *CopyCommand) Run(ctx context.Context, args []string) error {
	svc, list, err := storeArgs{env: c.env}.parse(ctx, args)
	if err != nil {
		return err
	}
	res, err := svc.Store.Copy(ctx, list[:len(list)-1], list[len(list)-1], c.opt)
	if err != nil {
		return err
	}
	c.env.Console.Printf("copied %d file(s) (%s)", res.Files, ui.Size(res.Bytes))
	return nil
}
