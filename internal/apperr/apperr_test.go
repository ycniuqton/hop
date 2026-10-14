package apperr

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodeOf(t *testing.T) {
	notFound := New(ExitNotFound, "%q not on shared shelf", "x")
	assert.Equal(t, ExitOK, CodeOf(nil))
	assert.Equal(t, ExitNotFound, CodeOf(notFound))
	assert.Equal(t, ExitNotFound, CodeOf(fmt.Errorf("wrapped: %w", notFound)))
	assert.Equal(t, ExitInterrupted, CodeOf(context.Canceled))
	assert.Equal(t, ExitGeneral, CodeOf(errors.New("boom")))
}

func TestErrorMessageWithHint(t *testing.T) {
	e := New(ExitNotFound, `"dmp.sql" not on shared shelf`).WithHint("did you mean dump.sql?")
	assert.Equal(t, `"dmp.sql" not on shared shelf — did you mean dump.sql?`, e.Error())
}
