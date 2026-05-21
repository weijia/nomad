// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	mdnsdiscovery "github.com/hashicorp/nomad/nomad/discovery"
)

// setupMDNS starts the mDNS service registration and discovery.
// This allows Nomad nodes to automatically discover each other on the
// local network without manual configuration.
func (c *Command) setupMDNS(config *Config) error {
	if config == nil || (!config.Server.Enabled && !config.Client.Enabled) {
		return nil
	}

	mdnsConfig := &mdnsdiscovery.MDNSConfig{
		Enabled:  true,
		HTTPPort: config.Ports.HTTP,
		RPCPort:  config.Ports.RPC,
		SerfPort: config.Ports.Serf,
	}

	mdnsDisc, err := mdnsdiscovery.NewMDNSDiscovery(mdnsConfig, c.agent.logger.Named("mdns"))
	if err != nil {
		c.agent.logger.Warn("failed to create mDNS discovery", "error", err)
		return nil // non-fatal
	}

	if err := mdnsDisc.Start(); err != nil {
		c.agent.logger.Warn("failed to start mDNS discovery", "error", err)
		return nil // non-fatal
	}

	c.mdnsDiscovery = mdnsDisc
	c.agent.logger.Info("mDNS discovery started",
		"instance", mdnsConfig.InstanceName,
		"http_port", mdnsConfig.HTTPPort,
		"rpc_port", mdnsConfig.RPCPort,
		"serf_port", mdnsConfig.SerfPort,
	)

	return nil
}

// stopMDNS shuts down the mDNS discovery service.
func (c *Command) stopMDNS() {
	if c.mdnsDiscovery != nil {
		c.mdnsDiscovery.Stop()
		c.agent.logger.Info("mDNS discovery stopped")
	}
}
