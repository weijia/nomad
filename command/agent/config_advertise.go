// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package agent

import "net"

// lookupBindAddress resolves the bind address to find suitable advertise IPs.
// When binding to 0.0.0.0, uses UDP trick to detect the actual LAN IP.
func lookupBindAddress(bind string) ([]net.IP, error) {
	switch bind {
	case "0.0.0.0", "":
		// Auto-detect the local LAN IP using UDP trick
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
