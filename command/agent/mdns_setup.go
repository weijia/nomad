// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"os"
	"strings"
	"time"

	mdnsdiscovery "github.com/hashicorp/nomad/nomad/discovery"
)

const (
	// delayedBootstrapTimeout is how long to wait for peer discovery
	// before falling back to single-node mode.
	delayedBootstrapTimeout = 30 * time.Second

	// discoveryCheckInterval is how often to check for discovered peers.
	discoveryCheckInterval = 5 * time.Second

	// joinRetryInterval is how often to attempt joining discovered peers.
	joinRetryInterval = 15 * time.Second
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
		mdnsConfig.InstanceName = hostname
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

// startDiscoveryJoiner implements the delayed bootstrap strategy:
// 1. Wait up to delayedBootstrapTimeout for peer discovery
// 2. If peers found → join them and bootstrap as multi-node cluster
// 3. If timeout with no peers → fall back to single-node mode (bootstrap_expect=1)
// 4. Continue periodic join attempts for newly discovered peers
func (c *Command) startDiscoveryJoiner() {
	if c.broadcastDiscovery == nil && c.mdnsDiscovery == nil {
		return
	}

	go func() {
		joined := make(map[string]bool)
		bootstrapDone := false

		// Phase 1: Wait for peer discovery with timeout
		c.agent.logger.Info("delayed bootstrap: waiting for peer discovery",
			"timeout", delayedBootstrapTimeout)

		discoveryTicker := time.NewTicker(discoveryCheckInterval)
		defer discoveryTicker.Stop()

		timeout := time.After(delayedBootstrapTimeout)

		for !bootstrapDone {
			select {
			case <-c.retryJoinErrCh:
				return
			case <-timeout:
				// Timeout: no peers found, fall back to single-node mode
				c.agent.logger.Info("delayed bootstrap: timeout, falling back to single-node mode")
				if c.agent.server != nil {
					c.agent.server.TryDelayedBootstrap(1)
				}
				bootstrapDone = true

			case <-discoveryTicker.C:
				addrs := c.getDiscoveryJoinAddresses()
				if len(addrs) == 0 {
					continue
				}

				// Found peers! Try to join them
				var newAddrs []string
				for _, addr := range addrs {
					if !joined[addr] {
						newAddrs = append(newAddrs, addr)
					}
				}

				if len(newAddrs) > 0 && c.agent.server != nil {
					c.agent.logger.Info("discovery joiner: attempting to join discovered nodes",
						"addrs", strings.Join(newAddrs, ","))

					n, err := c.agent.server.Join(newAddrs)
					if err != nil {
						c.agent.logger.Warn("discovery joiner: join failed", "error", err)
					} else if n > 0 {
						c.agent.logger.Info("discovery joiner: joined nodes", "count", n)
						for _, addr := range newAddrs {
							joined[addr] = true
						}

						// After successful join, trigger multi-node bootstrap
						peerCount := len(joined) + 1 // +1 for self
						c.agent.logger.Info("delayed bootstrap: peers found, triggering cluster bootstrap",
							"known_peers", peerCount)
						c.agent.server.TryDelayedBootstrap(peerCount)
						bootstrapDone = true
					}
				}
			}
		}

		// Phase 2: Continue periodic join for any new peers
		c.agent.logger.Info("discovery joiner: entering steady-state mode")
		joinTicker := time.NewTicker(joinRetryInterval)
		defer joinTicker.Stop()

		for {
			select {
			case <-c.retryJoinErrCh:
				return
			case <-joinTicker.C:
				addrs := c.getDiscoveryJoinAddresses()
				if len(addrs) == 0 {
					continue
				}

				var newAddrs []string
				for _, addr := range addrs {
					if !joined[addr] {
						newAddrs = append(newAddrs, addr)
					}
				}
				if len(newAddrs) == 0 {
					continue
				}

				if c.agent.server != nil {
					c.agent.logger.Info("discovery joiner: attempting to join new nodes",
						"addrs", strings.Join(newAddrs, ","))
					n, err := c.agent.server.Join(newAddrs)
					if err != nil {
						c.agent.logger.Warn("discovery joiner: join failed", "error", err)
					} else if n > 0 {
						c.agent.logger.Info("discovery joiner: joined new nodes", "count", n)
						for _, addr := range newAddrs {
							joined[addr] = true
						}
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
