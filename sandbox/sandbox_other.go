//go:build !openbsd

package sandbox

// Unveil is a no-op on non-OpenBSD systems.
// On OpenBSD, it adds a path to the list of unveiled paths with specified permissions.
func Unveil(path, perms string) error { return nil }

// UnveilBlock is a no-op on non-OpenBSD systems.
// On OpenBSD, it blocks future unveil calls and locks the unveil list.
func UnveilBlock() error { return nil }

// Pledge is a no-op on non-OpenBSD systems.
// On OpenBSD, it restricts the system calls available to the process.
func Pledge(promises string) error { return nil }
