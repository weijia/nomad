// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build android

package agent

import (
	"errors"
	"net"
)

// getInterfaceByName returns an interface on Android.
// Since net.InterfaceByName uses netlink which is restricted on Android,
// we only support common interface names without actually querying.
func getInterfaceByName(name string) (*net.Interface, error) {
	// On Android, we can't query interfaces, but we can accept common names
	switch name {
	case "lo", "lo0", "dummy0":
		// Return a fake interface for loopback
		return &net.Interface{
			Index: 1,
			MTU:   65536,
			Name:  name,
			Flags: net.FlagUp | net.FlagLoopback,
		}, nil
	case "wlan0", "eth0", "tun0":
		// Return a fake interface for common network interfaces
		return &net.Interface{
			Index: 2,
			MTU:   1500,
			Name:  name,
			Flags: net.FlagUp | net.FlagMulticast,
		}, nil
	default:
		// For any other name, return an error
		return nil, errors.New("interface lookup not supported on Android")
	}
}
