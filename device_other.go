//go:build !darwin

package main

// shouldScanSequentially on non-macOS platforms defaults to false (concurrent).
// Without platform-specific drive detection, we assume concurrent is safe.
func shouldScanSequentially(dir1, dir2 string) bool {
	return true
}
