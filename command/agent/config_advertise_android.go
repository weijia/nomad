// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build android

package agent

import "net"

// lookupBindAddress resolves the bind address to find suitable advertise IPs.
// On Android, when bind is 0.0.0.0, we auto-detect the LAN IP using a UDP trick.
func lookupBindAddress(bind string) ([]net.IP, error) {
	switch bind {
	case "0.0.0.0", "":
		// Auto-detect the local LAN IP
		ip, err := autoDetectLocalIP()
		if err != nil {
			// Fallback to loopback
			return []net.IP{net.ParseIP("127.0.0.1")}, nil
		}
		return []net.IP{ip}, nil
	case "127.0.0.1", "localhost":
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}

	// For specific IPs, return as-is
	if ip := net.ParseIP(bind); ip != nil {
		return []net.IP{ip}, nil
	}

	// Try DNS lookup as fallback
	return net.LookupIP(bind)
}
