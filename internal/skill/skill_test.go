package skill

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCRLFCheckoutIsNormalized(t *testing.T) {
	assert.Equal(t, "---\nname: hop\n---\n# hop\n", Source("---\r\nname: hop\r\n---\r\n# hop\r\n").Normalized())
	assert.NotContains(t, Document(), "\r")
	assert.True(t, strings.HasPrefix(Guidance(), "# hop"))
}
