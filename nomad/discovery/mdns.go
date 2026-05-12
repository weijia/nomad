// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/mdns"
)

const (
	// NomadServiceName is the mDNS service name for Nomad
	NomadServiceName = "_nomad._tcp"
	// NomadHTTPService is the mDNS service for Nomad HTTP API
	NomadHTTPService = "_nomad-http._tcp"
	// NomadRPCService is the mDNS service for Nomad RPC
	NomadRPCService = "_nomad-rpc._tcp"
	// NomadSerfService is the mDNS service for Nomad Serf
	NomadSerfService = "_nomad-serf._tcp"
)

// MDNSConfig holds configuration for mDNS discovery
type MDNSConfig struct {
	// Enabled controls whether mDNS is enabled
	Enabled bool

	// ServiceName is the base service name (default: "nomad")
	ServiceName string

	// InstanceName is a unique name for this instance
	InstanceName string

	// HTTPPort is the HTTP API port
	HTTPPort int

	// RPCPort is the RPC port
	RPCPort int

	// SerfPort is the Serf port
	SerfPort int

	// Domain is the mDNS domain (default: "local")
	Domain string

	// TTL is the time-to-live for mDNS records
	TTL time.Duration
}

// DefaultMDNSConfig returns a default mDNS configuration
func DefaultMDNSConfig() *MDNSConfig {
	return &MDNSConfig{
		Enabled:      true,
		ServiceName:  "nomad",
		InstanceName: "",
		HTTPPort:     4646,
		RPCPort:      4647,
		SerfPort:     4648,
		Domain:       "local",
		TTL:          120 * time.Second,
	}
}

// MDNSDiscovery handles mDNS service registration and discovery
type MDNSDiscovery struct {
	config     *MDNSConfig
	logger     hclog.Logger
	shutdownCh chan struct{}
	wg         sync.WaitGroup

	// servers holds the mDNS servers for each service
	servers []*mdns.Server

	// discoveredNodes holds recently discovered nodes
	discoveredNodes map[string]*DiscoveredNode
	nodeLock        sync.RWMutex
}

// DiscoveredNode represents a node discovered via mDNS
type DiscoveredNode struct {
	InstanceName string
	Service      string
	Host         string
	HTTPPort     int
	RPCPort      int
	SerfPort     int
	LastSeen     time.Time
	TXT          []string
}

// NewMDNSDiscovery creates a new mDNS discovery instance
func NewMDNSDiscovery(config *MDNSConfig, logger hclog.Logger) (*MDNSDiscovery, error) {
	if config == nil {
		config = DefaultMDNSConfig()
	}

	if config.InstanceName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "nomad-node"
		}
		config.InstanceName = fmt.Sprintf("%s-%d", hostname, time.Now().Unix())
	}

	d := &MDNSDiscovery{
		config:          config,
		logger:          logger.Named("mdns"),
		shutdownCh:      make(chan struct{}),
		discoveredNodes: make(map[string]*DiscoveredNode),
	}

	return d, nil
}

// Start begins mDNS service registration and discovery
func (d *MDNSDiscovery) Start() error {
	if !d.config.Enabled {
		d.logger.Info("mDNS discovery disabled")
		return nil
	}

	d.logger.Info("starting mDNS discovery", "instance", d.config.InstanceName)

	// Register services
	if err := d.registerServices(); err != nil {
		return fmt.Errorf("failed to register mDNS services: %w", err)
	}

	// Start discovery listener
	d.wg.Add(1)
	go d.discoveryLoop()

	return nil
}

// Stop shuts down mDNS discovery
func (d *MDNSDiscovery) Stop() error {
	if !d.config.Enabled {
		return nil
	}

	d.logger.Info("stopping mDNS discovery")
	close(d.shutdownCh)

	// Shutdown all mDNS servers
	for _, server := range d.servers {
		server.Shutdown()
	}

	d.wg.Wait()
	return nil
}

// registerServices registers all Nomad services via mDNS
func (d *MDNSDiscovery) registerServices() error {
	// Register HTTP service
	if d.config.HTTPPort > 0 {
		if err := d.registerService(NomadHTTPService, d.config.HTTPPort, []string{
			fmt.Sprintf("rpc_port=%d", d.config.RPCPort),
			fmt.Sprintf("serf_port=%d", d.config.SerfPort),
		}); err != nil {
			return err
		}
	}

	// Register RPC service
	if d.config.RPCPort > 0 {
		if err := d.registerService(NomadRPCService, d.config.RPCPort, []string{
			fmt.Sprintf("http_port=%d", d.config.HTTPPort),
			fmt.Sprintf("serf_port=%d", d.config.SerfPort),
		}); err != nil {
			return err
		}
	}

	// Register Serf service
	if d.config.SerfPort > 0 {
		if err := d.registerService(NomadSerfService, d.config.SerfPort, []string{
			fmt.Sprintf("http_port=%d", d.config.HTTPPort),
			fmt.Sprintf("rpc_port=%d", d.config.RPCPort),
		}); err != nil {
			return err
		}
	}

	return nil
}

// registerService registers a single mDNS service
func (d *MDNSDiscovery) registerService(service string, port int, txt []string) error {
	host, err := os.Hostname()
	if err != nil {
		host = "localhost"
	}

	info := &mdns.ServiceInfo{
		Name:   d.config.InstanceName,
		Host:   host,
		Port:   port,
		Info:   txt,
		TTL:    uint32(d.config.TTL.Seconds()),
		Domain: d.config.Domain,
	}

	server, err := mdns.NewServer(&mdns.Config{
		Zone: info,
	})
	if err != nil {
		return fmt.Errorf("failed to create mDNS server for %s: %w", service, err)
	}

	d.servers = append(d.servers, server)
	d.logger.Info("registered mDNS service",
		"service", service,
		"instance", d.config.InstanceName,
		"port", port,
	)

	return nil
}

// discoveryLoop continuously discovers other Nomad nodes
func (d *MDNSDiscovery) discoveryLoop() {
	defer d.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial discovery
	d.discoverServices()

	for {
		select {
		case <-d.shutdownCh:
			return
		case <-ticker.C:
			d.discoverServices()
			d.cleanupStaleNodes()
		}
	}
}

// discoverServices discovers all Nomad services on the network
func (d *MDNSDiscovery) discoverServices() {
	services := []string{NomadHTTPService, NomadRPCService, NomadSerfService}

	for _, service := range services {
		d.discoverService(service)
	}
}

// discoverService discovers a specific service
func (d *MDNSDiscovery) discoverService(service string) {
	entries := make(chan *mdns.ServiceEntry, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		for entry := range entries {
			d.handleDiscoveredEntry(entry, service)
		}
	}()

	params := &mdns.QueryParam{
		Service:             service,
		Domain:              d.config.Domain,
		Timeout:             5 * time.Second,
		Entries:             entries,
		DisableIPv6:         false,
		WantUnicastResponse: false,
	}

	if err := mdns.Query(params); err != nil {
		d.logger.Warn("mDNS query failed", "service", service, "error", err)
	}

	close(entries)
}

// handleDiscoveredEntry processes a discovered mDNS entry
func (d *MDNSDiscovery) handleDiscoveredEntry(entry *mdns.ServiceEntry, service string) {
	// Skip ourselves
	if entry.Name == d.config.InstanceName {
		return
	}

	host := entry.AddrV4.String()
	if host == "" {
		host = entry.AddrV6.String()
	}

	node := &DiscoveredNode{
		InstanceName: entry.Name,
		Service:      service,
		Host:         host,
		LastSeen:     time.Now(),
		TXT:          entry.InfoFields,
	}

	// Parse TXT records for additional ports
	for _, txt := range entry.InfoFields {
		var port int
		switch {
		case fmt.Sscanf(txt, "http_port=%d", &port) == 1:
			node.HTTPPort = port
		case fmt.Sscanf(txt, "rpc_port=%d", &port) == 1:
			node.RPCPort = port
		case fmt.Sscanf(txt, "serf_port=%d", &port) == 1:
			node.SerfPort = port
		}
	}

	// Set the port from the entry if not in TXT
	switch service {
	case NomadHTTPService:
		node.HTTPPort = entry.Port
	case NomadRPCService:
		node.RPCPort = entry.Port
	case NomadSerfService:
		node.SerfPort = entry.Port
	}

	key := fmt.Sprintf("%s-%s", entry.Name, service)

	d.nodeLock.Lock()
	oldNode, exists := d.discoveredNodes[key]
	d.discoveredNodes[key] = node
	d.nodeLock.Unlock()

	if !exists {
		d.logger.Info("discovered new Nomad node",
			"instance", entry.Name,
			"service", service,
			"host", host,
			"port", entry.Port,
		)
	} else {
		d.logger.Debug("updated Nomad node",
			"instance", entry.Name,
			"service", service,
			"host", host,
		)
	}

	_ = oldNode // Avoid unused variable warning
}

// cleanupStaleNodes removes nodes that haven't been seen recently
func (d *MDNSDiscovery) cleanupStaleNodes() {
	cutoff := time.Now().Add(-5 * time.Minute)

	d.nodeLock.Lock()
	defer d.nodeLock.Unlock()

	for key, node := range d.discoveredNodes {
		if node.LastSeen.Before(cutoff) {
			delete(d.discoveredNodes, key)
			d.logger.Info("removed stale node", "instance", node.InstanceName)
		}
	}
}

// GetDiscoveredNodes returns all currently discovered nodes
func (d *MDNSDiscovery) GetDiscoveredNodes() []*DiscoveredNode {
	d.nodeLock.RLock()
	defer d.nodeLock.RUnlock()

	nodes := make([]*DiscoveredNode, 0, len(d.discoveredNodes))
	for _, node := range d.discoveredNodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetDiscoveredHTTPNodes returns discovered nodes with HTTP endpoints
func (d *MDNSDiscovery) GetDiscoveredHTTPNodes() []*DiscoveredNode {
	d.nodeLock.RLock()
	defer d.nodeLock.RUnlock()

	nodes := make([]*DiscoveredNode, 0)
	seen := make(map[string]bool)

	for _, node := range d.discoveredNodes {
		if node.HTTPPort > 0 && !seen[node.InstanceName] {
			nodes = append(nodes, node)
			seen[node.InstanceName] = true
		}
	}
	return nodes
}

// JoinAddresses returns Serf join addresses for discovered nodes
func (d *MDNSDiscovery) JoinAddresses() []string {
	d.nodeLock.RLock()
	defer d.nodeLock.RUnlock()

	addresses := make([]string, 0)
	seen := make(map[string]bool)

	for _, node := range d.discoveredNodes {
		if node.SerfPort > 0 && !seen[node.Host] {
			addresses = append(addresses, fmt.Sprintf("%s:%d", node.Host, node.SerfPort))
			seen[node.Host] = true
		}
	}
	return addresses
}
