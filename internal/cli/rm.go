package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/ui"
)

// RemoveStoreCommand deletes store files or folders.
type RemoveStoreCommand struct {
	env       *Env
	recursive bool
	yes       bool
	from      string
}

func (c *RemoveStoreCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "rm [-r] <@device:path>...", Short: "Delete store files or folders", GroupID: string(GroupStore)}
	cmd.Flags().BoolVarP(&c.recursive, "recursive", "r", false, "delete folders")
	cmd.Flags().BoolVarP(&c.yes, "yes", "y", false, "do not ask for confirmation")
	cmd.ValidArgsFunction = env.completer().Store(false)
	return bindHandler(cmd, atLeast1, c)
}

func (c *RemoveStoreCommand) Run(ctx context.Context, args []string) error {
	svc, list, err := storeArgs{env: c.env}.parse(ctx, args)
	if err != nil {
		return err
	}
	c.from = list.Refs()
	n, err := svc.Store.Remove(ctx, list, c.recursive, c.confirm)
	if err != nil {
		return err
	}
	return c.env.Console.Done(ui.Outcome{Action: ui.Actions.Delete, Data: map[string]any{"deleted": n, "from": c.from}},
		"deleted %d object(s) from %s", n, c.from)
}

func (c *RemoveStoreCommand) confirm(n int) (bool, error) {
	if c.yes {
		return true, nil
	}
	return c.env.Console.Confirm(fmt.Sprintf("Delete %d objects from %s?", n, c.from), false)
}
