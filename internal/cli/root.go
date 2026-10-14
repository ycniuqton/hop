// Package cli wires Cobra commands. It parses arguments and delegates to workspace managers; it never calls S3.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/apperr"
)

// GlobalOptions holds the flags shared by every command.
type GlobalOptions struct {
	Quiet   bool
	JSON    bool
	NoColor bool
	Verbose bool
}

// Command is a class-based handler that contributes one Cobra command to the tree.
type Command interface {
	Build(env *Env) *cobra.Command
}

// App owns the root command and the registered command handlers.
type App struct {
	env      *Env
	commands []Command
}

// NewApp creates the application with every v1 command registered.
func NewApp(env *Env) *App {
	return &App{env: env, commands: registeredCommands()}
}

// Root builds the full Cobra command tree.
func (a *App) Root() *cobra.Command {
	root := &cobra.Command{
		Use:           "hop",
		Short:         "Move files between your own machines through one S3 bucket",
		Version:       a.env.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(*cobra.Command, []string) {
			a.env.applyOptions()
		},
		PersistentPostRun: func(cmd *cobra.Command, _ []string) {
			a.env.touch(cmd.Context())
		},
	}
	a.env.root = root
	root.SetVersionTemplate("hop {{.Version}}\n")
	root.SetIn(a.env.Console.In)
	root.SetOut(a.env.Console.Out)
	root.SetErr(a.env.Console.Err)
	a.bindGlobalFlags(root)
	for _, g := range CommandGroups() {
		root.AddGroup(&cobra.Group{ID: string(g), Title: g.Title()})
	}
	for _, c := range a.commands {
		root.AddCommand(c.Build(a.env))
	}
	return root
}

func (a *App) bindGlobalFlags(root *cobra.Command) {
	f := root.PersistentFlags()
	f.BoolVarP(&a.env.Opts.Quiet, "quiet", "q", false, "print nothing except errors")
	f.BoolVar(&a.env.Opts.JSON, "json", false, "machine-readable output for list commands")
	f.BoolVar(&a.env.Opts.NoColor, "no-color", false, "disable colored output")
	f.BoolVarP(&a.env.Opts.Verbose, "verbose", "v", false, "log S3 calls")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return apperr.New(apperr.ExitUsage, "%s", err.Error())
	})
}

// Run executes the command tree with Ctrl-C cancellation and returns the exit code.
func (a *App) Run(ctx context.Context, args []string) apperr.ExitCode {
	root := a.Root()
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	if err != nil && ctx.Err() == nil {
		fmt.Fprintf(a.env.Console.Err, "error: %s\n", err)
	}
	return exitCodeFor(ctx, err)
}

// exitCodeFor treats any error not produced by a handler as a Cobra usage failure.
func exitCodeFor(ctx context.Context, err error) apperr.ExitCode {
	var ae *apperr.Error
	switch {
	case ctx.Err() != nil:
		return apperr.ExitInterrupted
	case err != nil && !errors.As(err, &ae):
		return apperr.ExitUsage
	default:
		return apperr.CodeOf(err)
	}
}

// Execute is the binary entry point.
func Execute(version string) apperr.ExitCode {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return NewApp(NewEnv(version)).Run(ctx, os.Args[1:])
}
