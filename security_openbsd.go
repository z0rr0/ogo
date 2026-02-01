// +build openbsd

package main

import (
	"golang.org/x/sys/unix"
)

// setupSecurity applies OpenBSD-specific security restrictions using pledge and unveil
func setupSecurity(dir string) error {
	// unveil restricts filesystem access to only the specified directory
	// This allows the process to only access files within the served directory
	if err := unix.Unveil(dir, "r"); err != nil {
		return err
	}

	// Lock down unveil - no more unveil calls allowed
	if err := unix.UnveilBlock(); err != nil {
		return err
	}

	// pledge restricts system calls the process can make
	// "stdio" - standard I/O operations
	// "rpath" - read filesystem paths
	// "inet" - network operations (for HTTP server)
	// "dns" - DNS lookups (if needed)
	if err := unix.Pledge("stdio rpath inet dns", ""); err != nil {
		return err
	}

	return nil
}
