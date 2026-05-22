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
// It tries multiple locations and picks the first one that is writable.
func defaultDataDir() string {
	candidates := []string{}

	// On Android, /data/local/tmp is writable from root/shell
	if _, err := os.Stat("/data/local/tmp"); err == nil {
		candidates = append(candidates, "/data/local/tmp/nomad")
	}

	// $HOME/.nomad works on most Unix systems and Android app environments
	if home, err := os.UserHomeDir(); err == nil && home != "" && home != "/" {
		candidates = append(candidates, home+"/.nomad")
	}

	// $TMPDIR is available on most systems
	if tmp := os.Getenv("TMPDIR"); tmp != "" {
		candidates = append(candidates, tmp+"/nomad")
	}

	// Current directory as last resort
	candidates = append(candidates, "./data")

	for _, dir := range candidates {
		if isWritableDir(dir) {
			return dir
		}
	}

	// If nothing is writable, return ./data and let the caller handle the error
	return "./data"
}

// isWritableDir checks if a directory exists and is writable,
// or if it can be created.
func isWritableDir(dir string) bool {
	// Try to create the directory if it doesn't exist
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return false
		}
	}
	// Check if we can write to it
	f, err := os.CreateTemp(dir, ".nomad_test_")
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(f.Name())
	return true
}
