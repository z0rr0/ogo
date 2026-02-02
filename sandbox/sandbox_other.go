//go:build !openbsd

// Package sandbox provides platform-specific security restrictions using OpenBSD pledge/unveil.
package sandbox

import "log/slog"

// Unveil is a no-op on non-OpenBSD systems.
// On OpenBSD, it adds a path to the list of unveiled paths with specified permissions.
func Unveil(path, perms string) error {
	slog.Debug("unveil skip", "path", path, "perms", perms)
	return nil
}

// UnveilBlock is a no-op on non-OpenBSD systems.
// On OpenBSD, it blocks future unveil calls and locks the unveil list.
func UnveilBlock() error {
	slog.Debug("unveil block skip")
	return nil
}

// Pledge is a no-op on non-OpenBSD systems.
// On OpenBSD, it restricts the system calls available to the process.
func Pledge(promises string) error {
	slog.Debug("pledge skip", "promises", promises)
	return nil
}
