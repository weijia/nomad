// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package agent

import "os"

// applyAndroidDefaults applies zero-config defaults to the config.
// If server mode is enabled without explicit bootstrap_expect,
// default to 1 and enable mDNS auto-discovery for zero-config startup.
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

	// If no retry_join is configured, enable mDNS auto-discovery
	if len(cfg.Server.ServerJoin.RetryJoin) == 0 {
		cfg.Server.ServerJoin.RetryJoin = []string{"provider=mdns"}
	}
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
