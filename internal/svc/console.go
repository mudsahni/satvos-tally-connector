//go:build !windows

package svc

// IsWindowsService returns false on non-Windows platforms.
func IsWindowsService() bool {
	return false
}
