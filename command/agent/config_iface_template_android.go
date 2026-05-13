// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build android

package agent

import (
	"strings"
)

// parseInterfaceTemplate parses a go-sockaddr template on Android.
// Since go-sockaddr/template may call net.Interfaces() which uses netlink,
// we only support plain interface names without template syntax.
func parseInterfaceTemplate(tpl string) (string, error) {
	// If it's a plain interface name (no template syntax), return as-is
	if !strings.Contains(tpl, "{{") && !strings.Contains(tpl, "}}") {
		return strings.TrimSpace(tpl), nil
	}

	// Template syntax is not supported on Android due to netlink restrictions
	return "", nil
}
