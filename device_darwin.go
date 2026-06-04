//go:build darwin

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// getDeviceID returns the device ID (st_dev) for the given path.
func getDeviceID(path string) (int32, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, nil
	}
	return stat.Dev, nil
}

// getMountPoint returns the mount point for the given path using df.
func getMountPoint(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("df", "-P", absPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	lines := strings.Split(out.String(), "\n")
	if len(lines) < 2 {
		return "", nil
	}
	// The last field of the second line is the mount point.
	fields := strings.Fields(lines[1])
	if len(fields) < 6 {
		return "", nil
	}
	return fields[5], nil
}

// isSolidState checks whether the drive at the given mount point is an SSD
// by querying diskutil.
func isSolidState(mountPoint string) (bool, error) {
	cmd := exec.Command("diskutil", "info", mountPoint)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return false, err
	}
	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Solid State:") {
			return strings.Contains(line, "Yes"), nil
		}
	}
	// If not reported, assume SSD (safe for concurrency).
	return true, nil
}

// shouldScanSequentially returns true only when both directories are on the
// same physical device AND that device is a mechanical HDD.
// In all other cases it returns false (meaning: scan concurrently).
func shouldScanSequentially(dir1, dir2 string) bool {
	dev1, err1 := getDeviceID(dir1)
	dev2, err2 := getDeviceID(dir2)
	if err1 != nil || err2 != nil {
		// Can't determine – default to concurrent.
		return false
	}

	if dev1 != dev2 {
		// Different devices – always concurrent.
		return false
	}

	// Same device – check if it is a mechanical HDD.
	mount, err := getMountPoint(dir1)
	if err != nil || mount == "" {
		return false
	}

	ssd, err := isSolidState(mount)
	if err != nil {
		return false
	}

	// Sequential only when same device AND mechanical HDD.
	return !ssd
}
