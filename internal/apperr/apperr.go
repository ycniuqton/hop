// Package apperr defines hop's domain errors and the process exit codes they map to.
package apperr

import (
	"context"
	"errors"
	"fmt"
)

// ExitCode is the process exit status contract from spec section 10.
type ExitCode int

const (
	ExitOK          ExitCode = 0
	ExitGeneral     ExitCode = 1
	ExitUsage       ExitCode = 2
	ExitNotFound    ExitCode = 3
	ExitConflict    ExitCode = 4
	ExitAuth        ExitCode = 5
	ExitNetwork     ExitCode = 6
	ExitInterrupted ExitCode = 130
)

var kinds = map[ExitCode]string{
	ExitOK: "ok", ExitGeneral: "general", ExitUsage: "usage", ExitNotFound: "not_found", ExitConflict: "conflict",
	ExitAuth: "auth", ExitNetwork: "network", ExitInterrupted: "interrupted",
}

// Kind is the stable machine-readable name of an exit code, used in JSON errors.
func (c ExitCode) Kind() string {
	if k, ok := kinds[c]; ok {
		return k
	}
	return kinds[ExitGeneral]
}

// Error is a user-facing failure carrying its exit code and an optional fix hint.
type Error struct {
	Code ExitCode
	Msg  string
	Hint string
	Err  error
}

func (e *Error) Error() string {
	if e.Hint == "" {
		return e.Msg
	}
	return e.Msg + " — " + e.Hint
}

func (e *Error) Unwrap() error { return e.Err }

// New builds an Error with a formatted message.
func New(code ExitCode, format string, args ...any) *Error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, args...)}
}

// WithHint returns a copy of the error with the given fix hint.
func (e *Error) WithHint(format string, args ...any) *Error {
	c := *e
	c.Hint = fmt.Sprintf(format, args...)
	return &c
}

// Wrap attaches an underlying cause to the error.
func (e *Error) Wrap(err error) *Error {
	c := *e
	c.Err = err
	return &c
}

// CodeOf resolves the exit code for any error returned by a command.
func CodeOf(err error) ExitCode {
	var ae *Error
	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, context.Canceled):
		return ExitInterrupted
	case errors.As(err, &ae):
		return ae.Code
	default:
		return ExitGeneral
	}
}
