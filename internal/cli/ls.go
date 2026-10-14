package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/target"
	"github.com/ycniuqton/hop/internal/ui"
	"github.com/ycniuqton/hop/internal/workspace"
)

// ListCommand lists a store folder.
type ListCommand struct {
	env       *Env
	recursive bool
	long      bool
}

// DefaultListTarget is what `hop ls` lists with no argument: this device's store root.
const DefaultListTarget = "@:"

func (c *ListCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "ls [-R] [-l] [@device:path]", Short: "List a store folder", GroupID: string(GroupStore)}
	cmd.Flags().BoolVarP(&c.recursive, "recursive", "R", false, "list everything below the folder")
	cmd.Flags().BoolVarP(&c.long, "long", "l", false, "show size and modified time")
	cmd.ValidArgsFunction = env.completer().Store(false)
	return bindHandler(cmd, upToOne, c)
}

func (c *ListCommand) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		args = []string{DefaultListTarget}
	}
	svc, list, err := storeArgs{env: c.env}.parse(ctx, args)
	if err != nil {
		return err
	}
	if list[0].Kind != target.Remote {
		return apperr.New(apperr.ExitUsage, "ls lists store paths").WithHint("use @device:path, e.g. hop ls @:")
	}
	entries, err := svc.Store.List(ctx, list[0], c.recursive)
	if err != nil {
		return err
	}
	if c.env.Opts.JSON {
		return c.env.Console.PrintJSON(append([]workspace.Entry{}, entries...))
	}
	c.print(entries)
	return nil
}

func (c *ListCommand) print(entries []workspace.Entry) {
	if !c.long {
		for _, e := range entries {
			c.env.Console.Printf("%s", e.Name)
		}
		return
	}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		size, mod := "-", "-"
		if !e.Dir {
			size, mod = ui.Size(e.Size), ui.Ago(c.env.Now(), e.Modified)
		}
		rows[i] = []string{size, mod, e.Name}
	}
	c.env.Console.Table([]string{"SIZE", "MODIFIED", "NAME"}, rows)
}
