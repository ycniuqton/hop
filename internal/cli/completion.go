package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/complete"
)

// CompletionCommand prints a completion script, or installs it with `completion install`.
type CompletionCommand struct {
	env *Env
}

func (c *CompletionCommand) Build(env *Env) *cobra.Command {
	c.env = env
	cmd := &cobra.Command{Use: "completion <" + strings.Join(complete.Shells(), "|") + ">", Short: "Install or print shell completion",
		GroupID: string(GroupSetup), ValidArgs: complete.Shells()}
	cmd.AddCommand((&CompletionInstallCommand{}).Build(env))
	return bindHandler(cmd, oneArg, c)
}

func (c *CompletionCommand) Run(_ context.Context, args []string) error {
	_, inst, err := complete.Lookup(args[0])
	if err != nil {
		return err
	}
	return inst.Script(c.env.root, c.env.Console.Out)
}

// CompletionInstallCommand installs completion for the shell in $SHELL.
type CompletionInstallCommand struct {
	env *Env
}

func (c *CompletionInstallCommand) Build(env *Env) *cobra.Command {
	c.env = env
	return bindHandler(&cobra.Command{Use: "install", Short: "Install completion for your shell ($SHELL)"}, noArgs, c)
}

func (c *CompletionInstallCommand) Run(context.Context, []string) error {
	_, inst, err := complete.Detect(c.env.Getenv(EnvVars.Shell))
	if err != nil {
		return err
	}
	path, err := complete.Install(c.env.root, inst, c.env.home())
	if err != nil {
		return err
	}
	c.env.Console.Successf("installed %s — open a new shell to use it", path)
	if hint := inst.Hint(path, c.env.Getenv); hint != "" {
		c.env.Console.Printf("%s", hint)
	}
	return nil
}
