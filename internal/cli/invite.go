package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/joincode"
)

// InviteCommand prints the join command for a new machine.
type InviteCommand struct {
	env      *Env
	codeOnly bool
}

func (c *InviteCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "invite", Short: "Print the join command for another machine", GroupID: string(GroupSetup)}
	cmd.Flags().BoolVar(&c.codeOnly, "code", false, "print only the code, for scripts")
	return bindHandler(cmd, noArgs, c)
}

func (c *InviteCommand) Run(context.Context, []string) error {
	cfg, err := c.env.Config()
	if err != nil {
		return err
	}
	code := joincode.Current.Encode(joincode.Payload{
		Bucket: cfg.Bucket, Region: cfg.Region, Prefix: cfg.Prefix, Endpoint: cfg.Endpoint,
		AccessKeyID: cfg.AccessKeyID, SecretAccessKey: cfg.SecretAccessKey, WorkspaceID: cfg.WorkspaceID,
	})
	out := c.env.Console.Out
	if c.codeOnly {
		fmt.Fprintln(out, code)
		return nil
	}
	fmt.Fprintln(out, "Run this on the new machine (it contains your S3 keys — treat it like a password):")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  hop join %s\n", code)
	return nil
}
