//go:build openbsd

// Package sandbox provides sandboxing functions for OpenBSD.
package sandbox

import (
	"log/slog"

	"golang.org/x/sys/unix"
)

// Unveil adds a path to the list of unveiled paths with specified permissions.
// The perms string can contain r (read), w (write), x (execute), and c (create).
func Unveil(path, perms string) error {
	slog.Debug("unveil", "path", path, "perms", perms)
	return unix.Unveil(path, perms)
}

// UnveilBlock blocks future unveil calls and locks the unveil list.
func UnveilBlock() error {
	slog.Debug("unveil block")
	return unix.UnveilBlock()
}

// Pledge restricts the system calls available to the process.
// The promises string contains space-separated pledge promises.
// The unix.Pledge 2nd parameter is empty string, because the process doesn't use exec syscalls.
func Pledge(promises string) error {
	slog.Debug("pledge", "promises", promises)
	return unix.Pledge(promises, "")
}
