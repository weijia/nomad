// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package agent

// applyAndroidDefaults applies zero-config defaults to the config.
// If server mode is enabled without explicit bootstrap_expect,
// default to 1 and enable mDNS auto-discovery for zero-config startup.
func applyAndroidDefaults(cfg *Config) {
	if cfg == nil || cfg.Server == nil || !cfg.Server.Enabled {
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
