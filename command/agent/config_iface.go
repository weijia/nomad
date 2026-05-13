// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !android

package agent

import "net"

// getInterfaceByName wraps net.InterfaceByName.
// On non-Android platforms, it uses the standard net package.
func getInterfaceByName(name string) (*net.Interface, error) {
	return net.InterfaceByName(name)
}
