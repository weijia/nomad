// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build android

package agent

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// autoDetectLocalIP uses a UDP connection trick to find the local IP
// that would be used to reach the internet. This does NOT require netlink
// or any special permissions on Android.
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

// parseSingleIPTemplate parses an IP template on Android.
// Android restricts netlink socket access, so we avoid template parsing
// for plain IP addresses. Only simple IPs and loopback are supported.
func parseSingleIPTemplate(tpl string) (string, error) {
	// If it's already a plain IP, return it directly
	if ip := net.ParseIP(tpl); ip != nil {
		return tpl, nil
	}

	// Handle common template patterns without netlink
	switch strings.TrimSpace(tpl) {
	case "127.0.0.1", "localhost", "::1":
		return "127.0.0.1", nil
	case "0.0.0.0", "*", "":
		return "0.0.0.0", nil
	}

	// Check for template syntax (contains {{ or }})
	if strings.Contains(tpl, "{{") || strings.Contains(tpl, "}}") {
		// On Android, we can't resolve interface IPs due to netlink restrictions
		// Return an error suggesting to use a specific IP address
		return "", fmt.Errorf("template %q requires network interface query which is not supported on Android; use a specific IP address like 127.0.0.1 or 0.0.0.0", tpl)
	}

	// Try to resolve as hostname (may fail on Android without DNS)
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

	// Fallback to first IP (IPv6)
	return ips[0].String(), nil
}

// isPlainIP checks if the string is a plain IP address without template syntax.
func isPlainIP(s string) bool {
	return net.ParseIP(s) != nil
}

// getDefaultAdvertiseAddr returns the default advertise address on Android.
// Uses UDP trick to auto-detect the local LAN IP without netlink.
// Falls back to 127.0.0.1 if detection fails.
func getDefaultAdvertiseAddr() (string, error) {
	ip, err := autoDetectLocalIP()
	if err != nil {
		return "127.0.0.1", nil
	}
	return ip.String(), nil
}
