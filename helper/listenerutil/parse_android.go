// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build android

package listenerutil

import (
	"fmt"
	"net"
	"strings"
)

// ParseAddressTemplate parses an address template on Android.
// Since listenerutil.ParseSingleIPTemplate uses netlink which is restricted,
// we handle common cases without netlink.
func ParseAddressTemplate(addr string) (string, error) {
	// Handle empty string
	if addr == "" {
		return "", nil
	}

	// Check for template syntax
	if strings.Contains(addr, "{{") || strings.Contains(addr, "}}") {
		return "", fmt.Errorf("template syntax not supported on Android")
	}

	// Try to parse as host:port
	host, port, err := net.SplitHostPort(addr)
	if err == nil {
		// It's a host:port format
		// Resolve host if it's a hostname
		if ip := net.ParseIP(host); ip != nil {
			return addr, nil
		}

		// Handle common hostnames
		switch host {
		case "localhost":
			return "127.0.0.1:" + port, nil
		}

		// Try DNS lookup
		ips, err := net.LookupIP(host)
		if err != nil {
			return "", fmt.Errorf("failed to resolve %q: %w", host, err)
		}
		if len(ips) > 0 {
			return ips[0].String() + ":" + port, nil
		}
	}

	// Try as plain IP
	if ip := net.ParseIP(addr); ip != nil {
		return addr, nil
	}

	// Handle common hostnames without port
	switch addr {
	case "localhost":
		return "127.0.0.1", nil
	}

	// Try DNS lookup
	ips, err := net.LookupIP(addr)
	if err != nil {
		return "", fmt.Errorf("failed to resolve %q: %w", addr, err)
	}
	if len(ips) > 0 {
		return ips[0].String(), nil
	}

	return addr, nil
}
