package naming

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeviceValidate(t *testing.T) {
	for _, ok := range []string{"web-1", "a", "0db", "laptop", "a234567890123456789012345678901b"} {
		assert.NoError(t, Devices.Validate(ok), ok)
	}
	for _, bad := range []string{"", "-web", "Web", "web_1", "web.1", "_shared", "me", "a2345678901234567890123456789012x"} {
		assert.Error(t, Devices.Validate(bad), bad)
	}
}

func TestFromHostname(t *testing.T) {
	cases := map[string]string{
		"ip-10-0-1-12.ec2.internal": "ip-10-0-1-12",
		"My_Laptop":                 "my-laptop",
		"--weird":                   "weird",
		"me":                        "device",
		"":                          "device",
	}
	for in, want := range cases {
		assert.Equal(t, want, Devices.FromHostname(in), in)
	}
}

func TestHotNameValidate(t *testing.T) {
	assert.NoError(t, HotNames.Validate("dump.sql"))
	assert.ErrorContains(t, HotNames.Validate("a/b"), "hot names are flat")
	assert.Error(t, HotNames.Validate(""))
	assert.Error(t, HotNames.Validate(".."))
}
