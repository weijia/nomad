// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net"
	"os"
	"runtime"

	clientconfig "github.com/hashicorp/nomad/client/config"
)

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

	// Apply server-specific defaults if server mode is enabled
	if cfg.Server != nil && cfg.Server.Enabled {
		// Do NOT set bootstrap_expect here. The delayed bootstrap mechanism
		// in startDiscoveryJoiner will handle it:
		// - If peers are discovered via broadcast/mDNS, bootstrap as a multi-node cluster
		// - If no peers are found within the timeout, fall back to single-node mode
		// This avoids the split-brain problem where two nodes independently bootstrap.

		// Peer discovery is handled by the built-in broadcast/mDNS discovery
		// mechanism (setupDiscovery), not by go-discover's retry_join. The go-discover
		// mDNS provider doesn't work on Windows due to multicast limitations.
		// Do NOT set RetryJoin here — discoveryJoiner handles it directly.
	}

	// Apply client-specific defaults if client mode is enabled
	if cfg.Client != nil && cfg.Client.Enabled {
		// If client has no servers configured but server is enabled on same node,
		// configure client to connect to local server
		if len(cfg.Client.Servers) == 0 && cfg.Server != nil && cfg.Server.Enabled {
			// Client will connect to local server via RPC port
			// The address will be resolved during client startup
			cfg.Client.Servers = []string{"127.0.0.1:4647"}
		}

		// On Windows, if no network interface is specified, try to find a suitable one
		if cfg.Client.NetworkInterface == "" && runtime.GOOS == "windows" {
			if iface := findWindowsNetworkInterface(); iface != "" {
				cfg.Client.NetworkInterface = iface
			}
		}

		// Increase GC thresholds to avoid premature allocation cleanup.
		// Default disk threshold (80%) is too aggressive for development,
		// causing batch job logs to be deleted immediately after completion.
		//
		// Note: convertClientConfig reads from cfg.ClientConfig (client.Config),
		// not cfg.Client (ClientConfig). We need to set both to ensure the
		// values are properly passed through.
		if cfg.Client.GCDiskUsageThreshold == 0 {
			cfg.Client.GCDiskUsageThreshold = 95
		}
		if cfg.Client.GCInodeUsageThreshold == 0 {
			cfg.Client.GCInodeUsageThreshold = 95
		}
		if cfg.Client.GCMaxAllocs == 0 {
			cfg.Client.GCMaxAllocs = 500
		}

		// Also set on ClientConfig which is what convertClientConfig actually uses
		if cfg.ClientConfig == nil {
			cfg.ClientConfig = clientconfig.DefaultConfig()
		}
		if cfg.ClientConfig.GCDiskUsageThreshold == 0 {
			cfg.ClientConfig.GCDiskUsageThreshold = 95
		}
		if cfg.ClientConfig.GCInodeUsageThreshold == 0 {
			cfg.ClientConfig.GCInodeUsageThreshold = 95
		}
		if cfg.ClientConfig.GCMaxAllocs == 0 {
			cfg.ClientConfig.GCMaxAllocs = 500
		}
	}
}

// findWindowsNetworkInterface attempts to find a suitable network interface on Windows.
// It looks for common interface names or the first interface with a valid IP.
func findWindowsNetworkInterface() string {
	// Try common interface names first
	commonNames := []string{"以太网", "Ethernet", "Wi-Fi", "WiFi", "WLAN", "本地连接", "Local Area Connection"}

	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	// First pass: look for common names that are UP and have a non-APIPA IPv4 address
	for _, iface := range interfaces {
		// Skip interfaces that are not up
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		for _, name := range commonNames {
			if iface.Name == name {
				addrs, err := iface.Addrs()
				if err != nil || len(addrs) == 0 {
					continue
				}
				// Check if this interface has a real (non-APIPA, non-link-local) IPv4 address
				if hasRealIPv4(addrs) {
					return iface.Name
				}
			}
		}
	}

	// Second pass: return the first up interface with a real IPv4 address (excluding loopback)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}
		if hasRealIPv4(addrs) {
			return iface.Name
		}
	}

	return ""
}

// hasRealIPv4 checks if any address is a real (non-APIPA, non-link-local) IPv4 address
func hasRealIPv4(addrs []net.Addr) bool {
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.To4() == nil {
			continue
		}
		// Skip APIPA (169.254.x.x) and link-local
		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			continue
		}
		// Skip APIPA range
		if ip[0] == 169 && ip[1] == 254 {
			continue
		}
		return true
	}
	return false
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
