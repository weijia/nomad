// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build android

package agent

import (
	"net"
)

// lookupBindAddress resolves the bind address to find suitable advertise IPs.
// On Android, we avoid DNS lookups and return appropriate defaults.
func lookupBindAddress(bind string) ([]net.IP, error) {
	// Handle special bind addresses
	switch bind {
	case "0.0.0.0", "":
		// Return loopback as the advertise address
		// This is safe for Android where we typically run in single-node mode
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	case "127.0.0.1", "localhost":
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}

	// For specific IPs, return as-is
	if ip := net.ParseIP(bind); ip != nil {
		return []net.IP{ip}, nil
	}

	// Try DNS lookup as fallback (may fail on Android)
	return net.LookupIP(bind)
}
