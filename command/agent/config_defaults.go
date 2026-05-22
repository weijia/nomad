// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package agent

import "os"

// applyAndroidDefaults applies zero-config defaults to the config.
// If server mode is enabled without explicit bootstrap_expect,
// default to 1 for single-node startup. Also enables mDNS
// auto-discovery for finding peers on the local network.
func applyAndroidDefaults(cfg *Config) {
	if cfg == nil {
		return
	}

	// If data_dir is not set, use a platform-appropriate default
	if cfg.DataDir == "" {
		cfg.DataDir = defaultDataDir()
	}

	// Only apply server-specific defaults if server mode is enabled
	if cfg.Server == nil || !cfg.Server.Enabled {
		return
	}

	// If bootstrap_expect is not explicitly set, default to 1
	if cfg.Server.BootstrapExpect == 0 {
		cfg.Server.BootstrapExpect = 1
	}

	// Peer discovery is handled by the built-in broadcast/mDNS discovery
	// mechanism (setupMDNS), not by go-discover's retry_join. The go-discover
	// mDNS provider doesn't work on Windows due to multicast limitations.
	// Do NOT set RetryJoin here — discoveryJoiner handles it directly.
}

// defaultDataDir returns the default data directory based on the platform.
func defaultDataDir() string {
	// On Android (app environment), use current directory (app's files dir)
	// On Android (root/shell), use /data/local/tmp/nomad
	// On other Unix systems, use $HOME/.nomad
	// On Windows, use %LOCALAPPDATA%\Nomad
	if _, err := os.Stat("/system/build.prop"); err == nil {
		// Android system - check if we have permission to write to /data/local/tmp
		if _, err := os.Stat("/data/local/tmp"); err == nil {
			// Try to create a test file to check permissions
			testFile := "/data/local/tmp/.nomad_test_" + string(os.Getpid())
			if f, err := os.Create(testFile); err == nil {
				f.Close()
				os.Remove(testFile)
				return "/data/local/tmp/nomad"
			}
		}
		// No permission - use current directory ( app's private dir)
		return "./data"
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home + "/.nomad"
	}
	return "./data"
}
