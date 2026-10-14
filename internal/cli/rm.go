package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// RemoveStoreCommand deletes store files or folders.
type RemoveStoreCommand struct {
	env       *Env
	recursive bool
	yes       bool
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
	n, err := svc.Store.Remove(ctx, list, c.recursive, c.confirm)
	if err != nil {
		return err
	}
	c.env.Console.Successf("deleted %d object(s)", n)
	return nil
}

func (c *RemoveStoreCommand) confirm(n int) (bool, error) {
	if c.yes {
		return true, nil
	}
	return c.env.Console.Confirm(fmt.Sprintf("Delete %d objects?", n), false)
}
