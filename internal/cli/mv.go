package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/ui"
)

// MoveCommand moves store files or folders.
type MoveCommand struct {
	env   *Env
	force bool
}

func (c *MoveCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "mv <src> <dst>", Short: "Move a store file or folder", GroupID: string(GroupStore),
		Example: "  hop mv @web-1:a.sql @web-1:old/a.sql"}
	cmd.Flags().BoolVarP(&c.force, "force", "f", false, "overwrite existing files")
	cmd.ValidArgsFunction = env.completer().Store(false)
	return bindHandler(cmd, twoArgs, c)
}

func (c *MoveCommand) Run(ctx context.Context, args []string) error {
	svc, list, err := storeArgs{env: c.env}.parse(ctx, args)
	if err != nil {
		return err
	}
	res, err := svc.Store.Move(ctx, list[0], list[1], c.force)
	if err != nil {
		return err
	}
	return c.env.Console.Done(ui.Outcome{Action: ui.Actions.Move, Data: map[string]any{"files": res.Files}}, "moved %d file(s)", res.Files)
}
