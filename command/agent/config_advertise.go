// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !android

package agent

import "net"

// lookupBindAddress resolves the bind address to find suitable advertise IPs.
// On non-Android platforms, it uses net.LookupIP.
func lookupBindAddress(bind string) ([]net.IP, error) {
	return net.LookupIP(bind)
}
