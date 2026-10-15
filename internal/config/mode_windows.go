//go:build windows

package config

import "io/fs"

// platformForbidMask is zero on Windows: file modes do not describe ACLs there, and the config
// lives in the user's private profile.
const platformForbidMask fs.FileMode = 0
