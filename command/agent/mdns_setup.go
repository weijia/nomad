// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"os"
	"strings"
	"time"

	mdnsdiscovery "github.com/hashicorp/nomad/nomad/discovery"
)

// setupDiscovery starts peer discovery for automatic cluster formation.
// It always starts broadcast discovery (works everywhere) and optionally
// starts mDNS discovery (may not work on Windows due to multicast limits).
// The discoveryJoiner goroutine monitors both and drives server.Join().
func (c *Command) setupDiscovery(config *Config) error {
	if config == nil || !config.Server.Enabled {
		return nil
	}

	mdnsConfig := &mdnsdiscovery.MDNSConfig{
		Enabled:  true,
		HTTPPort: config.Ports.HTTP,
		RPCPort:  config.Ports.RPC,
		SerfPort: config.Ports.Serf,
	}

	// Ensure InstanceName is set before creating any discovery instance,
	// so both broadcast and mDNS share the same identity.
	if mdnsConfig.InstanceName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "nomad-node"
		}
		mdnsConfig.InstanceName = fmt.Sprintf("%s-%d", hostname, time.Now().Unix())
	}

	// Always start broadcast discovery — it works on all platforms
	broadcastDisc, err := mdnsdiscovery.NewBroadcastDiscovery(mdnsConfig, c.agent.logger.Named("broadcast"))
	if err != nil {
		c.agent.logger.Warn("failed to create broadcast discovery", "error", err)
	} else if err := broadcastDisc.Start(); err != nil {
		c.agent.logger.Warn("failed to start broadcast discovery", "error", err)
	} else {
		c.broadcastDiscovery = broadcastDisc
		c.agent.logger.Info("broadcast discovery started",
			"instance", mdnsConfig.InstanceName,
			"port", mdnsdiscovery.BroadcastPort,
		)
	}

	// Also try mDNS as a supplementary discovery mechanism
	mdnsDisc, err := mdnsdiscovery.NewMDNSDiscovery(mdnsConfig, c.agent.logger.Named("mdns"))
	if err != nil {
		c.agent.logger.Warn("failed to create mDNS discovery", "error", err)
	} else if err := mdnsDisc.Start(); err != nil {
		c.agent.logger.Warn("mDNS discovery failed to start (broadcast still active)", "error", err)
	} else {
		c.mdnsDiscovery = mdnsDisc
		c.agent.logger.Info("mDNS discovery started (supplementary to broadcast)",
			"instance", mdnsConfig.InstanceName,
		)
	}

	return nil
}

// startDiscoveryJoiner launches a background goroutine that periodically
// checks discovered peers and attempts to join them via server.Join().
// This replaces the go-discover retry_join mechanism which doesn't work
// on Windows due to multicast limitations.
func (c *Command) startDiscoveryJoiner() {
	if c.broadcastDiscovery == nil && c.mdnsDiscovery == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		joined := make(map[string]bool)

		for {
			select {
			case <-c.retryJoinErrCh:
				return
			case <-ticker.C:
			}

			addrs := c.getDiscoveryJoinAddresses()
			if len(addrs) == 0 {
				continue
			}

			// Filter out already-joined addresses
			var newAddrs []string
			for _, addr := range addrs {
				if !joined[addr] {
					newAddrs = append(newAddrs, addr)
				}
			}
			if len(newAddrs) == 0 {
				continue
			}

			c.agent.logger.Info("discovery joiner: attempting to join discovered nodes",
				"addrs", strings.Join(newAddrs, ","))

			if c.agent.server != nil {
				n, err := c.agent.server.Join(newAddrs)
				if err != nil {
					c.agent.logger.Warn("discovery joiner: join failed", "error", err)
				} else if n > 0 {
					c.agent.logger.Info("discovery joiner: joined nodes", "count", n)
					for _, addr := range newAddrs {
						joined[addr] = true
					}
				}
			}
		}
	}()
}

// stopDiscovery shuts down all discovery services.
func (c *Command) stopDiscovery() {
	if c.mdnsDiscovery != nil {
		c.mdnsDiscovery.Stop()
		c.agent.logger.Info("mDNS discovery stopped")
	}
	if c.broadcastDiscovery != nil {
		c.broadcastDiscovery.Stop()
		c.agent.logger.Info("broadcast discovery stopped")
	}
}

// getDiscoveryJoinAddresses returns addresses from broadcast and/or mDNS discovery
func (c *Command) getDiscoveryJoinAddresses() []string {
	var addrs []string

	// Collect from both sources
	if c.broadcastDiscovery != nil {
		addrs = append(addrs, c.broadcastDiscovery.JoinAddresses()...)
	}
	if c.mdnsDiscovery != nil {
		addrs = append(addrs, c.mdnsDiscovery.JoinAddresses()...)
	}

	return addrs
}
