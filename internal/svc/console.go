//go:build !windows

package svc

import "context"

// IsWindowsService returns false on non-Windows platforms.
func IsWindowsService() bool {
	return false
}

// Run is a no-op on non-Windows platforms. It should never be called because
// IsWindowsService always returns false, but exists to satisfy the compiler.
func Run(runFunc func(ctx context.Context) error) error {
	return runFunc(context.Background())
}
