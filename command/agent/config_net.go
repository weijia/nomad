// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !android

package agent

import (
	"net"

	"github.com/hashicorp/go-secure-stdlib/listenerutil"
)

// parseSingleIPTemplate wraps listenerutil.ParseSingleIPTemplate.
// On non-Android platforms, it uses the standard template parsing which may
// query network interfaces.
func parseSingleIPTemplate(tpl string) (string, error) {
	return listenerutil.ParseSingleIPTemplate(tpl)
}

// isPlainIP checks if the string is a plain IP address without template syntax.
func isPlainIP(s string) bool {
	return net.ParseIP(s) != nil
}

// getDefaultAdvertiseAddr returns the default advertise address.
// On non-Android platforms, it uses the first private IP.
func getDefaultAdvertiseAddr() (string, error) {
	return listenerutil.ParseSingleIPTemplate("{{ GetPrivateIP }}")
}
