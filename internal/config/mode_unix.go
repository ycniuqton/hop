//go:build !windows

package config

import "io/fs"

// platformForbidMask refuses configs readable by group or others.
const platformForbidMask fs.FileMode = 0o077
