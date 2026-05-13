// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build android

package agent

import (
	"errors"
	"net"
	"strings"
)

// parseMultipleIPTemplate parses multiple IP addresses from a template on Android.
// Since go-sockaddr/template may call net.Interfaces() which uses netlink,
// we only support plain IP addresses or comma/space-separated lists.
func parseMultipleIPTemplate(ipTmpl string) ([]string, error) {
	// If no template syntax, treat as plain addresses
	if !strings.Contains(ipTmpl, "{{") && !strings.Contains(ipTmpl, "}}") {
		// Split by space or comma
		var ips []string
		for _, sep := range []string{" ", ","} {
			if strings.Contains(ipTmpl, sep) {
				for _, ip := range strings.Split(ipTmpl, sep) {
					ip = strings.TrimSpace(ip)
					if ip != "" {
						ips = append(ips, ip)
					}
				}
				return deduplicateAddrs(ips), nil
			}
		}

		// Single address
		ip := strings.TrimSpace(ipTmpl)
		if ip == "" {
			return []string{}, errors.New("no addresses found")
		}

		// Validate it's an IP
		if net.ParseIP(ip) == nil {
			return []string{}, errors.New("invalid IP address")
		}

		return []string{ip}, nil
	}

	// Template syntax is not supported on Android
	return []string{}, errors.New("template syntax not supported on Android")
}
