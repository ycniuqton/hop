package suggest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClosest(t *testing.T) {
	names := []string{"web-1", "db-1", "laptop"}
	assert.Equal(t, "web-1", DidYouMean.Closest("web1", names))
	assert.Equal(t, "dump.sql", DidYouMean.Closest("dmp.sql", []string{"dump.sql", "x"}))
	assert.Equal(t, "", DidYouMean.Closest("zzzzzz", names))
	assert.Equal(t, "", DidYouMean.Closest("web-1", []string{"web-1"}))
	assert.Equal(t, 3, DidYouMean.distance("kitten", "sitting"))
}
