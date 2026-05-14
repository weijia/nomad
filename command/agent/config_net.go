// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/go-secure-stdlib/listenerutil"
)

// autoDetectLocalIP uses a UDP connection trick to find the local IP
// that would be used to reach the internet. This works on all platforms
// without requiring netlink or special permissions.
func autoDetectLocalIP() (net.IP, error) {
	// Connect a UDP socket to a public IP (doesn't actually send data).
	// The OS will choose the best local interface to reach that destination.
	conn, err := net.DialTimeout("udp4", "8.8.8.8:80", 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to detect local IP: %w", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	if localAddr.IP == nil || localAddr.IP.IsUnspecified() {
		return nil, fmt.Errorf("detected unspecified local IP")
	}

	return localAddr.IP, nil
}

// parseSingleIPTemplate parses an IP template.
// For plain IPs, returns directly. For template syntax, uses the standard
// listenerutil parser.
func parseSingleIPTemplate(tpl string) (string, error) {
	// If it's already a plain IP, return it directly
	if ip := net.ParseIP(tpl); ip != nil {
		return tpl, nil
	}

	// Handle common patterns
	switch strings.TrimSpace(tpl) {
	case "127.0.0.1", "localhost", "::1":
		return "127.0.0.1", nil
	case "0.0.0.0", "*", "":
		return "0.0.0.0", nil
	}

	// Check for template syntax (contains {{ or }})
	if strings.Contains(tpl, "{{") || strings.Contains(tpl, "}}") {
		return listenerutil.ParseSingleIPTemplate(tpl)
	}

	// Try to resolve as hostname
	ips, err := net.LookupIP(tpl)
	if err != nil {
		return "", fmt.Errorf("failed to resolve %q: %w", tpl, err)
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("no IP addresses found for %q", tpl)
	}

	// Return first IPv4 address
	for _, ip := range ips {
		if ip4 := ip.To4(); ip4 != nil {
			return ip4.String(), nil
		}
	}

	return ips[0].String(), nil
}

// isPlainIP checks if the string is a plain IP address without template syntax.
func isPlainIP(s string) bool {
	return net.ParseIP(s) != nil
}

// getDefaultAdvertiseAddr returns the default advertise address.
// Uses UDP trick to auto-detect the local LAN IP.
// Falls back to 127.0.0.1 if detection fails.
func getDefaultAdvertiseAddr() (string, error) {
	ip, err := autoDetectLocalIP()
	if err != nil {
		return "127.0.0.1", nil
	}
	return ip.String(), nil
}
