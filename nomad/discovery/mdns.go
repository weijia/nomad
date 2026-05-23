// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
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

	// Register services (broadcast our presence to other nodes)
	// On some platforms (e.g., Windows), service registration may fail due to
	// network restrictions. If all services fail, we should return an error
	// so the caller can fall back to alternative discovery methods.
	if err := d.registerServices(); err != nil {
		d.logger.Error("mDNS service registration failed",
			"error", err,
			"note", "falling back to alternative discovery method")
		return fmt.Errorf("mDNS registration failed: %w", err)
	}

	// Start discovery listener (find other nodes)
	d.wg.Add(1)
	go d.discoveryLoop()

	return nil
}

// Stop shuts down the mDNS discovery
func (d *MDNSDiscovery) Stop() {
	d.logger.Info("stopping mDNS discovery")
	close(d.shutdownCh)
	d.wg.Wait()

	// Shutdown all servers
	for _, server := range d.servers {
		server.Shutdown()
	}
}

// JoinAddresses returns addresses of discovered nodes for Serf joining
func (d *MDNSDiscovery) JoinAddresses() []string {
	d.nodeLock.RLock()
	defer d.nodeLock.RUnlock()

	var addrs []string
	now := time.Now()
	for _, node := range d.discoveredNodes {
		// Only return nodes seen in the last 5 minutes
		if now.Sub(node.LastSeen) < 5*time.Minute {
			addr := fmt.Sprintf("%s:%d", node.Host, node.SerfPort)
			addrs = append(addrs, addr)
		}
	}
	return addrs
}

// registerServices registers all Nomad services via mDNS
func (d *MDNSDiscovery) registerServices() error {
	host, err := os.Hostname()
	if err != nil {
		host = "localhost"
	}

	// mDNS requires a fully-qualified domain name (FQDN) ending with a period
	// Ensure the hostname is in FQDN format: hostname.local.
	if !strings.HasSuffix(host, ".") {
		host = host + ".local."
	}

	// Get the advertise address for the host
	ips := []net.IP{}
	if advertiseIP, err := getAdvertiseIP(); err == nil {
		ips = append(ips, advertiseIP)
	}

	// Create TXT records with port information
	httpTxt := []string{
		fmt.Sprintf("http_port=%d", d.config.HTTPPort),
		fmt.Sprintf("rpc_port=%d", d.config.RPCPort),
		fmt.Sprintf("serf_port=%d", d.config.SerfPort),
	}

	// Register RPC service
	rpcService, err := mdns.NewMDNSService(
		d.config.InstanceName,
		NomadRPCService,
		d.config.Domain,
		host,
		d.config.RPCPort,
		ips,
		httpTxt,
	)
	if err != nil {
		return fmt.Errorf("failed to create RPC mDNS service: %w", err)
	}

	rpcServer, err := mdns.NewServer(&mdns.Config{Zone: rpcService})
	if err != nil {
		d.logger.Warn("failed to create mDNS server for RPC", "error", err)
		// Continue with other services
	} else {
		d.servers = append(d.servers, rpcServer)
	}

	// Register Serf service
	serfService, err := mdns.NewMDNSService(
		d.config.InstanceName,
		NomadSerfService,
		d.config.Domain,
		host,
		d.config.SerfPort,
		ips,
		httpTxt,
	)
	if err != nil {
		return fmt.Errorf("failed to create Serf mDNS service: %w", err)
	}

	serfServer, err := mdns.NewServer(&mdns.Config{Zone: serfService})
	if err != nil {
		d.logger.Warn("failed to create mDNS server for Serf", "error", err)
		// Continue with other services
	} else {
		d.servers = append(d.servers, serfServer)
	}

	// Register HTTP service
	httpService, err := mdns.NewMDNSService(
		d.config.InstanceName,
		NomadHTTPService,
		d.config.Domain,
		host,
		d.config.HTTPPort,
		ips,
		httpTxt,
	)
	if err != nil {
		return fmt.Errorf("failed to create HTTP mDNS service: %w", err)
	}

	httpServer, err := mdns.NewServer(&mdns.Config{Zone: httpService})
	if err != nil {
		d.logger.Warn("failed to create mDNS server for HTTP", "error", err)
		// Continue - at least we tried
	} else {
		d.servers = append(d.servers, httpServer)
	}

	// If no servers were successfully created, return an error
	if len(d.servers) == 0 {
		return fmt.Errorf("failed to create any mDNS servers - network may not support multicast")
	}

	d.logger.Info("registered mDNS services",
		"instance", d.config.InstanceName,
		"http", d.config.HTTPPort,
		"rpc", d.config.RPCPort,
		"serf", d.config.SerfPort,
		"servers_started", len(d.servers),
	)

	return nil
}

// getAdvertiseIP returns the IP address to advertise
func getAdvertiseIP() (net.IP, error) {
	// Use UDP trick to get the actual LAN IP
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(8, 8, 8, 8), Port: 53})
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return parseIP(conn.LocalAddr().String())
}

// parseIP extracts IP from "host:port" string
func parseIP(addr string) (net.IP, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	return net.ParseIP(host), nil
}

// discoveryLoop continuously discovers other Nomad nodes
func (d *MDNSDiscovery) discoveryLoop() {
	defer d.wg.Done()

	// On Windows, mDNS query (multicast listen) often fails due to network
	// restrictions. Skip discovery loop - we still broadcast our presence via
	// registered services, and rely on broadcast discovery for finding peers.
	if runtime.GOOS == "windows" {
		d.logger.Debug("mDNS discovery loop disabled on Windows, service registration only")
		<-d.shutdownCh
		return
	}

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

	go func() {
		for entry := range entries {
			d.handleDiscoveredEntry(entry, service)
		}
	}()

	params := &mdns.QueryParam{
		Service:     service,
		Domain:      d.config.Domain,
		Timeout:     5 * time.Second,
		Entries:     entries,
		DisableIPv4: false,
		DisableIPv6: false,
	}

	if err := mdns.Query(params); err != nil {
		d.logger.Debug("mDNS query failed", "service", service, "error", err)
	}

	close(entries)
}

// handleDiscoveredEntry processes a discovered mDNS entry
func (d *MDNSDiscovery) handleDiscoveredEntry(entry *mdns.ServiceEntry, service string) {
	// Skip ourselves
	if entry.Name == d.config.InstanceName {
		return
	}

	// Get host address
	host := ""
	if entry.AddrV4 != nil {
		host = entry.AddrV4.String()
	} else if entry.AddrV6 != nil {
		host = entry.AddrV6.String()
	} else if entry.Addr != nil {
		host = entry.Addr.String()
	}

	if host == "" {
		return
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
		if n, _ := fmt.Sscanf(txt, "http_port=%d", &port); n == 1 {
			node.HTTPPort = port
		}
		if n, _ := fmt.Sscanf(txt, "rpc_port=%d", &port); n == 1 {
			node.RPCPort = port
		}
		if n, _ := fmt.Sscanf(txt, "serf_port=%d", &port); n == 1 {
			node.SerfPort = port
		}
	}

	// Set the port from the entry if not in TXT
	switch service {
	case NomadHTTPService:
		if node.HTTPPort == 0 {
			node.HTTPPort = entry.Port
		}
	case NomadRPCService:
		if node.RPCPort == 0 {
			node.RPCPort = entry.Port
		}
	case NomadSerfService:
		if node.SerfPort == 0 {
			node.SerfPort = entry.Port
		}
	}

	key := fmt.Sprintf("%s-%s", entry.Name, service)

	d.nodeLock.Lock()
	_, exists := d.discoveredNodes[key]
	d.discoveredNodes[key] = node
	d.nodeLock.Unlock()

	if !exists {
		d.logger.Info("discovered new Nomad node",
			"instance", entry.Name,
			"service", service,
			"host", host,
			"serf_port", node.SerfPort,
		)
	}
}

// cleanupStaleNodes removes nodes that haven't been seen recently
func (d *MDNSDiscovery) cleanupStaleNodes() {
	d.nodeLock.Lock()
	defer d.nodeLock.Unlock()

	now := time.Now()
	for key, node := range d.discoveredNodes {
		if now.Sub(node.LastSeen) > 10*time.Minute {
			delete(d.discoveredNodes, key)
			d.logger.Debug("removed stale node", "instance", node.InstanceName)
		}
	}
}

// GetDiscoveredNodes returns all discovered nodes
func (d *MDNSDiscovery) GetDiscoveredNodes() []*DiscoveredNode {
	d.nodeLock.RLock()
	defer d.nodeLock.RUnlock()

	nodes := make([]*DiscoveredNode, 0, len(d.discoveredNodes))
	for _, node := range d.discoveredNodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// HasDiscoveredNodes returns true if any nodes have been discovered
func (d *MDNSDiscovery) HasDiscoveredNodes() bool {
	d.nodeLock.RLock()
	defer d.nodeLock.RUnlock()
	return len(d.discoveredNodes) > 0
}
