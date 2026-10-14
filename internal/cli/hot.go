package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/apperr"
	"github.com/ycniuqton/hop/internal/complete"
	"github.com/ycniuqton/hop/internal/ui"
	"github.com/ycniuqton/hop/internal/workspace"
)

// HotCommand lists a hot shelf, and owns the rm and clear subcommands.
type HotCommand struct {
	env *Env
	all bool
}

func (c *HotCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "hot [@device|@]", Short: "List a hot shelf", GroupID: string(GroupHot)}
	cmd.Flags().BoolVar(&c.all, "all", false, "list every shelf, grouped")
	cmd.ValidArgsFunction = env.completer().Hot(complete.HotShelfOnly)
	cmd.AddCommand((&HotRemoveCommand{}).Build(env), (&HotClearCommand{}).Build(env))
	return bindHandler(cmd, upToOne, c)
}

func (c *HotCommand) Run(ctx context.Context, args []string) error {
	inv, err := hotArgs{env: c.env}.parse(ctx, args)
	if err != nil {
		return err
	}
	if len(inv.others) > 0 {
		return apperr.New(apperr.ExitUsage, "hot takes only an optional @device").WithHint("did you mean 'hop pull %s'?", inv.others[0].Raw)
	}
	shelves := []workspace.Shelf{inv.shelf}
	if c.all {
		if shelves, err = inv.svc.Shelves.Shelves(ctx); err != nil {
			return err
		}
	}
	var all []workspace.HotFile
	for i, sh := range shelves {
		files, err := inv.svc.Shelves.List(ctx, sh, true)
		if err != nil {
			return err
		}
		all = append(all, files...)
		c.print(sh, files, c.all, i > 0)
	}
	if c.env.Opts.JSON {
		return c.env.Console.PrintJSON(append([]workspace.HotFile{}, all...))
	}
	return nil
}

func (c *HotCommand) print(sh workspace.Shelf, files []workspace.HotFile, grouped, gap bool) {
	con := c.env.Console
	if con.JSON || con.Quiet {
		return
	}
	if gap {
		fmt.Fprintln(con.Out)
	}
	if grouped {
		fmt.Fprintf(con.Out, "%s:\n", sh.Label())
	}
	if len(files) == 0 {
		con.Printf("%s is empty", sh.Label())
		return
	}
	rows := make([][]string, len(files))
	for i, f := range files {
		rows[i] = []string{f.Name, ui.Size(f.Size), f.From, ui.Ago(c.env.Now(), f.Modified)}
	}
	con.Table([]string{"NAME", "SIZE", "FROM", "AGE"}, rows)
}

// HotRemoveCommand deletes named files from a shelf.
type HotRemoveCommand struct {
	env *Env
}

func (c *HotRemoveCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "rm <name>... [@device|@]", Short: "Delete files from a hot shelf"}
	cmd.ValidArgsFunction = env.completer().HotRemove()
	return bindHandler(cmd, atLeast1, c)
}

func (c *HotRemoveCommand) Run(ctx context.Context, args []string) error {
	inv, err := hotArgs{env: c.env}.parse(ctx, args)
	if err != nil {
		return err
	}
	names := make([]string, len(inv.others))
	for i, t := range inv.others {
		names[i] = t.Path
	}
	if len(names) == 0 {
		return apperr.New(apperr.ExitUsage, "name at least one file to delete")
	}
	if err := inv.svc.Shelves.Remove(ctx, inv.shelf, names); err != nil {
		return err
	}
	return c.env.Console.Done(ui.Outcome{Action: ui.Actions.HotRemove, Data: map[string]any{"shelf": inv.shelf, "deleted": names}},
		"deleted %d file(s) from %s", len(names), inv.shelf.Label())
}

// HotClearCommand deletes everything on a shelf.
type HotClearCommand struct {
	env *Env
	yes bool
}

func (c *HotClearCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "clear [@device|@]", Short: "Delete every file on a hot shelf"}
	cmd.Flags().BoolVarP(&c.yes, "yes", "y", false, "do not ask for confirmation")
	cmd.ValidArgsFunction = env.completer().Hot(complete.HotShelfOnly)
	return bindHandler(cmd, upToOne, c)
}

func (c *HotClearCommand) Run(ctx context.Context, args []string) error {
	inv, err := hotArgs{env: c.env}.parse(ctx, args)
	if err != nil {
		return err
	}
	if !c.yes {
		if ok, err := c.env.Console.Confirm("Delete every file on "+inv.shelf.Label()+"?", false); err != nil || !ok {
			return err
		}
	}
	n, err := inv.svc.Shelves.Clear(ctx, inv.shelf)
	if err != nil {
		return err
	}
	return c.env.Console.Done(ui.Outcome{Action: ui.Actions.HotClear, Data: map[string]any{"shelf": inv.shelf, "deleted": n}},
		"deleted %d file(s) from %s", n, inv.shelf.Label())
}
