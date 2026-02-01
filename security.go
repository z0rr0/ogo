// +build !openbsd

package main

// setupSecurity is a no-op on non-OpenBSD systems
func setupSecurity(dir string) error {
	return nil
}
