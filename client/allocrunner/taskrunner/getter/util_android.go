// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build android

package getter

import (
	"os"
	"path/filepath"

	log "github.com/hashicorp/go-hclog"
	"golang.org/x/sys/unix"
)

// findHomeDir returns the home directory for Android.
func findHomeDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	if unix.Getuid() == 0 {
		return "/root"
	}
	return "/data/local/tmp"
}

// findConfigDir returns the config directory for Android.
func findConfigDir() string {
	config, err := os.UserConfigDir()
	if err == nil {
		return config
	}
	return filepath.Join(findHomeDir(), ".config")
}

// defaultEnvironment is the default minimal environment variables for Android.
func defaultEnvironment(taskDir string) map[string]string {
	tmpDir := filepath.Join(taskDir, "tmp")
	homeDir := findHomeDir()
	return map[string]string{
		"PATH":   "/system/bin:/usr/bin:/bin",
		"TMPDIR": tmpDir,
		"HOME":   homeDir,
	}
}

// lockdownAvailable returns false on Android (landlock not available).
func lockdownAvailable() bool {
	return false
}

// lockdown is a no-op on Android.
func lockdown(l log.Logger, allocDir, taskDir string, extra []string) error {
	// landlock is not available on Android
	return nil
}
