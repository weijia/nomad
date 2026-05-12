// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"fmt"
	"time"
)

// MDNSConfig holds configuration for mDNS-based service discovery
type MDNSConfig struct {
	// Enabled controls whether mDNS discovery is enabled
	Enabled bool `hcl:"enabled"`

	// ServiceName is the base service name for mDNS (default: "nomad")
	ServiceName string `hcl:"service_name"`

	// InstanceName is a unique name for this instance (default: hostname)
	InstanceName string `hcl:"instance_name"`

	// Domain is the mDNS domain (default: "local")
	Domain string `hcl:"domain"`

	// TTL is the time-to-live for mDNS records in seconds (default: 120)
	TTL int `hcl:"ttl"`

	// DiscoveryInterval is how often to search for other nodes (default: 30s)
	DiscoveryInterval time.Duration `hcl:"discovery_interval"`

	// EnableRegistration controls whether to register services (default: true)
	EnableRegistration bool `hcl:"enable_registration"`

	// EnableDiscovery controls whether to discover other nodes (default: true)
	EnableDiscovery bool `hcl:"enable_discovery"`
}

// DefaultMDNSConfig returns a default mDNS configuration
func DefaultMDNSConfig() *MDNSConfig {
	return &MDNSConfig{
		Enabled:            false,
		ServiceName:        "nomad",
		Domain:             "local",
		TTL:                120,
		DiscoveryInterval:  30 * time.Second,
		EnableRegistration: true,
		EnableDiscovery:    true,
	}
}

// Merge merges two MDNS configurations
func (m *MDNSConfig) Merge(other *MDNSConfig) *MDNSConfig {
	if other == nil {
		return m
	}

	result := *m

	if other.Enabled {
		result.Enabled = other.Enabled
	}
	if other.ServiceName != "" {
		result.ServiceName = other.ServiceName
	}
	if other.InstanceName != "" {
		result.InstanceName = other.InstanceName
	}
	if other.Domain != "" {
		result.Domain = other.Domain
	}
	if other.TTL != 0 {
		result.TTL = other.TTL
	}
	if other.DiscoveryInterval != 0 {
		result.DiscoveryInterval = other.DiscoveryInterval
	}
	if !other.EnableRegistration {
		result.EnableRegistration = other.EnableRegistration
	}
	if !other.EnableDiscovery {
		result.EnableDiscovery = other.EnableDiscovery
	}

	return &result
}

// Validate validates the MDNS configuration
func (m *MDNSConfig) Validate() error {
	if m.TTL < 10 {
		return fmt.Errorf("mDNS TTL must be at least 10 seconds")
	}
	if m.DiscoveryInterval < 5*time.Second {
		return fmt.Errorf("mDNS discovery interval must be at least 5 seconds")
	}
	return nil
}

// Copy returns a copy of the MDNS configuration
func (m *MDNSConfig) Copy() *MDNSConfig {
	if m == nil {
		return nil
	}
	return &MDNSConfig{
		Enabled:            m.Enabled,
		ServiceName:        m.ServiceName,
		InstanceName:       m.InstanceName,
		Domain:             m.Domain,
		TTL:                m.TTL,
		DiscoveryInterval:  m.DiscoveryInterval,
		EnableRegistration: m.EnableRegistration,
		EnableDiscovery:    m.EnableDiscovery,
	}
}
