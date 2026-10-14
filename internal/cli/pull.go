package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/complete"
	"github.com/ycniuqton/hop/internal/ui"
)

// PullCommand downloads a file from a hot shelf.
type PullCommand struct {
	env    *Env
	remove bool
	force  bool
}

func (c *PullCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "pull <name> [@device|@] [dest]", Short: "Download a file from a hot shelf", GroupID: string(GroupHot),
		Example: "  hop pull dump.sql\n  hop pull dump.sql @\n  hop pull dump.sql @web-1 --rm"}
	cmd.Flags().BoolVar(&c.remove, "rm", false, "delete from the shelf after a successful download")
	cmd.Flags().BoolVarP(&c.force, "force", "f", false, "overwrite an existing local file")
	cmd.ValidArgsFunction = env.completer().Hot(complete.HotNames)
	return bindHandler(cmd, anyArgs, c)
}

func (c *PullCommand) Run(ctx context.Context, args []string) error {
	inv, err := hotArgs{env: c.env}.parse(ctx, args)
	if err != nil {
		return err
	}
	if len(inv.others) > 2 {
		return apperr.New(apperr.ExitUsage, "pull takes one name and an optional destination")
	}
	name, dest := "", ""
	if len(inv.others) > 0 {
		name = inv.others[0].Path
	}
	if len(inv.others) == 2 {
		dest = inv.others[1].Path
	}
	if name == "" {
		if name, err = c.pick(ctx, inv); err != nil {
			return err
		}
	}
	got, err := inv.svc.Shelves.Pull(ctx, inv.shelf, name, dest, c.force, c.remove)
	if err != nil {
		return err
	}
	c.env.Console.Successf("pulled %s → %s", name, got)
	return nil
}

func (c *PullCommand) pick(ctx context.Context, inv hotInvocation) (string, error) {
	if !c.env.Console.CanPrompt() {
		return "", apperr.New(apperr.ExitUsage, "missing file name").WithHint("run 'hop hot' to see what is on the shelf")
	}
	files, err := inv.svc.Shelves.List(ctx, inv.shelf, false)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", apperr.New(apperr.ExitNotFound, "%s is empty", inv.shelf.Label())
	}
	items := make([]string, len(files))
	for i, f := range files {
		items[i] = f.Name + "  (" + ui.Size(f.Size) + ", " + ui.Ago(c.env.Now(), f.Modified) + ")"
	}
	i, err := c.env.Console.Pick("Files on "+inv.shelf.Label()+":", items)
	if err != nil {
		return "", err
	}
	return files[i].Name, nil
}
