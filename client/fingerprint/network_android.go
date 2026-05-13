// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build android

package fingerprint

import (
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

// NetworkFingerprint is a stub for Android where net.Interfaces() is not available.
type NetworkFingerprint struct {
	StaticFingerprinter
	logger log.Logger
}

// Reload is a no-op but implements ReloadableFingerprint
func (f *NetworkFingerprint) Reload() {}

// NewNetworkFingerprint returns a stub NetworkFingerprinter for Android.
func NewNetworkFingerprint(logger log.Logger) Fingerprint {
	return &NetworkFingerprint{logger: logger.Named("network")}
}

func (f *NetworkFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	// On Android, we cannot enumerate network interfaces due to netlink restrictions.
	// Return a minimal network configuration with loopback.

	// Initialize NodeResources if needed
	if resp.NodeResources == nil {
		resp.NodeResources = &structs.NodeResources{
			Networks: []*structs.NetworkResource{},
		}
	}

	// Add a minimal loopback network entry
	newNetwork := &structs.NetworkResource{
		Mode:   "host",
		Device: "lo",
		CIDR:   "127.0.0.1/32",
		IP:     "127.0.0.1",
		MBits:  DefaultNetworkSpeed,
	}

	resp.AddAttribute("unique.network.ip", newNetwork.IP)
	resp.NodeResources.Networks = append(resp.NodeResources.Networks, newNetwork)
	resp.Detected = true

	return nil
}
