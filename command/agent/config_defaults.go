// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package agent

import "os"

// applyAndroidDefaults applies zero-config defaults to the config.
// If server mode is enabled without explicit bootstrap_expect,
// default to 1 for single-node startup.
func applyAndroidDefaults(cfg *Config) {
	if cfg == nil {
		return
	}

	// If data_dir is not set, use a platform-appropriate default
	if cfg.DataDir == "" {
		cfg.DataDir = defaultDataDir()
	}

	if cfg.Server == nil || !cfg.Server.Enabled {
		return
	}

	// If bootstrap_expect is not explicitly set, default to 1
	if cfg.Server.BootstrapExpect == 0 {
		cfg.Server.BootstrapExpect = 1
	}

	// Note: mDNS auto-discovery via "provider=mdns" is not yet integrated
	// with the retry_join mechanism. To join a cluster, either:
	// 1. Specify peer IPs manually: -retry-join="192.168.1.100"
	// 2. Use a supported go-discover provider (aws, gce, etc.)
}

// defaultDataDir returns the default data directory based on the platform.
func defaultDataDir() string {
	// On Android, use /data/local/tmp/nomad
	// On other Unix systems, use /var/lib/nomad or $HOME/.nomad
	// On Windows, use %LOCALAPPDATA%\Nomad
	if _, err := os.Stat("/data/local/tmp"); err == nil {
		return "/data/local/tmp/nomad"
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home + "/.nomad"
	}
	return "./nomad-data"
}
