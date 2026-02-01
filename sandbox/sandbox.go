//go:build openbsd

// Package sandbox provides sandboxing functions for OpenBSD.
package sandbox

import "golang.org/x/sys/unix"

// Unveil adds a path to the list of unveiled paths with specified permissions.
// The perms string can contain r (read), w (write), x (execute), and c (create).
func Unveil(path, perms string) error { return unix.Unveil(path, perms) }

// UnveilBlock blocks future unveil calls and locks the unveil list.
func UnveilBlock() error { return unix.UnveilBlock() }

// Pledge restricts the system calls available to the process.
// The promises string contains space-separated pledge promises.
func Pledge(promises string) error { return unix.Pledge(promises, "") }
