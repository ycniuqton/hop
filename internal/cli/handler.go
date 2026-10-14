package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/ycniuqton/hop/internal/apperr"
)

// Handler is the business entry of one command; it receives parsed positional arguments.
type Handler interface {
	Run(ctx context.Context, args []string) error
}

// ArgRule validates positional argument count before a handler runs.
type ArgRule struct {
	Min int
	Max int
}

// Unlimited marks an ArgRule without an upper bound.
const Unlimited = -1

var (
	anyArgs  = ArgRule{Min: 0, Max: Unlimited}
	noArgs   = ArgRule{Min: 0, Max: 0}
	upToOne  = ArgRule{Min: 0, Max: 1}
	oneArg   = ArgRule{Min: 1, Max: 1}
	twoArgs  = ArgRule{Min: 2, Max: 2}
	atLeast1 = ArgRule{Min: 1, Max: Unlimited}
	atLeast2 = ArgRule{Min: 2, Max: Unlimited}
)

func (r ArgRule) validate(_ *cobra.Command, args []string) error {
	n := len(args)
	if n < r.Min {
		return apperr.New(apperr.ExitUsage, "expected at least %d argument(s), got %d", r.Min, n)
	}
	if r.Max != Unlimited && n > r.Max {
		return apperr.New(apperr.ExitUsage, "expected at most %d argument(s), got %d", r.Max, n)
	}
	return nil
}

// bindHandler attaches a handler so that every failure it returns carries an exit code.
func bindHandler(c *cobra.Command, rule ArgRule, h Handler) *cobra.Command {
	c.Args = rule.validate
	c.RunE = func(cmd *cobra.Command, args []string) error {
		return asAppError(h.Run(cmd.Context(), args))
	}
	return c
}

func asAppError(err error) error {
	var ae *apperr.Error
	if err == nil || errors.As(err, &ae) || errors.Is(err, context.Canceled) {
		return err
	}
	return apperr.New(apperr.ExitGeneral, "%s", err.Error()).Wrap(err)
}
