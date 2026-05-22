// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
)

const (
	// BroadcastPort is the UDP port used for broadcast discovery
	BroadcastPort = 4649
	// BroadcastInterval is how often we broadcast our presence
	BroadcastInterval = 10 * time.Second
	// BroadcastTimeout is the read timeout for UDP socket
	BroadcastTimeout = 2 * time.Second
	// NodeTimeout is how long before a node is considered stale
	NodeTimeout = 30 * time.Second
)

// BroadcastMessage is the structure sent via UDP broadcast
type BroadcastMessage struct {
	InstanceName string `json:"instance"`
	HTTPPort     int    `json:"http_port"`
	RPCPort      int    `json:"rpc_port"`
	SerfPort     int    `json:"serf_port"`
	Address      string `json:"address"`
	Timestamp    int64  `json:"timestamp"`
}

// BroadcastDiscovery handles UDP broadcast-based service discovery
// This is an alternative to mDNS for environments where multicast is not available
type BroadcastDiscovery struct {
	config     *MDNSConfig
	logger     hclog.Logger
	shutdownCh chan struct{}
	conn       *net.UDPConn

	// Use the same node tracking as MDNS
	discoveredNodes map[string]*DiscoveredNode
	nodeLock        sync.RWMutex
	wg              sync.WaitGroup
}

// NewBroadcastDiscovery creates a new broadcast discovery instance
func NewBroadcastDiscovery(config *MDNSConfig, logger hclog.Logger) (*BroadcastDiscovery, error) {
	if config == nil {
		config = DefaultMDNSConfig()
	}

	d := &BroadcastDiscovery{
		config:          config,
		logger:          logger.Named("broadcast"),
		shutdownCh:      make(chan struct{}),
		discoveredNodes: make(map[string]*DiscoveredNode),
	}

	return d, nil
}

// Start begins broadcast-based discovery
func (d *BroadcastDiscovery) Start() error {
	if !d.config.Enabled {
		d.logger.Info("broadcast discovery disabled")
		return nil
	}

	d.logger.Info("starting broadcast discovery", "instance", d.config.InstanceName, "port", BroadcastPort)

	// Create UDP socket for broadcast
	addr := &net.UDPAddr{Port: BroadcastPort}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return fmt.Errorf("failed to create UDP socket: %w", err)
	}
	d.conn = conn

	// Start broadcaster
	d.wg.Add(1)
	go d.broadcastLoop()

	// Start listener
	d.wg.Add(1)
	go d.listenLoop()

	// Start cleanup routine
	d.wg.Add(1)
	go d.cleanupLoop()

	d.logger.Info("broadcast discovery started", "address", conn.LocalAddr().String())
	return nil
}

// Stop shuts down broadcast discovery
func (d *BroadcastDiscovery) Stop() {
	d.logger.Info("stopping broadcast discovery")
	close(d.shutdownCh)
	if d.conn != nil {
		d.conn.Close()
	}
	d.wg.Wait()
}

// broadcastLoop periodically broadcasts our presence
func (d *BroadcastDiscovery) broadcastLoop() {
	defer d.wg.Done()

	// Get our IP address
	advertiseIP, err := getAdvertiseIP()
	if err != nil {
		d.logger.Warn("failed to get advertise IP, using localhost", "error", err)
		advertiseIP = net.ParseIP("127.0.0.1")
	}

	msg := BroadcastMessage{
		InstanceName: d.config.InstanceName,
		HTTPPort:     d.config.HTTPPort,
		RPCPort:      d.config.RPCPort,
		SerfPort:     d.config.SerfPort,
		Address:      advertiseIP.String(),
	}

	ticker := time.NewTicker(BroadcastInterval)
	defer ticker.Stop()

	// Broadcast immediately on start
	d.broadcast(msg)

	for {
		select {
		case <-d.shutdownCh:
			return
		case <-ticker.C:
			msg.Timestamp = time.Now().Unix()
			d.broadcast(msg)
		}
	}
}

// broadcast sends a single broadcast message
func (d *BroadcastDiscovery) broadcast(msg BroadcastMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		d.logger.Warn("failed to marshal broadcast message", "error", err)
		return
	}

	// Broadcast to subnet (255.255.255.255)
	addr := &net.UDPAddr{
		IP:   net.ParseIP("255.255.255.255"),
		Port: BroadcastPort,
	}

	_, err = d.conn.WriteToUDP(data, addr)
	if err != nil {
		d.logger.Debug("broadcast send failed", "error", err)
	} else {
		d.logger.Debug("broadcast sent", "instance", msg.InstanceName, "address", msg.Address)
	}
}

// listenLoop receives broadcast messages from other nodes
func (d *BroadcastDiscovery) listenLoop() {
	defer d.wg.Done()

	buf := make([]byte, 1024)
	for {
		select {
		case <-d.shutdownCh:
			return
		default:
		}

		// Set read timeout to allow checking shutdownCh
		d.conn.SetReadDeadline(time.Now().Add(BroadcastTimeout))
		n, remoteAddr, err := d.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // Timeout is expected, loop back and check shutdown
			}
			// Other errors
			select {
			case <-d.shutdownCh:
				return
			default:
				d.logger.Debug("read error", "error", err)
				continue
			}
		}

		// Parse message
		var msg BroadcastMessage
		if err := json.Unmarshal(buf[:n], &msg); err != nil {
			d.logger.Debug("failed to unmarshal broadcast message", "error", err)
			continue
		}

		// Skip ourselves
		if msg.InstanceName == d.config.InstanceName {
			continue
		}

		// Record discovered node
		d.handleDiscoveredNode(&msg, remoteAddr)
	}
}

// handleDiscoveredNode processes a discovered node
func (d *BroadcastDiscovery) handleDiscoveredNode(msg *BroadcastMessage, remoteAddr *net.UDPAddr) {
	// Use the advertised address if available, otherwise use the remote address
	host := msg.Address
	if host == "" || host == "127.0.0.1" {
		host = remoteAddr.IP.String()
	}

	node := &DiscoveredNode{
		InstanceName: msg.InstanceName,
		Service:      NomadSerfService,
		Host:         host,
		HTTPPort:     msg.HTTPPort,
		RPCPort:      msg.RPCPort,
		SerfPort:     msg.SerfPort,
		LastSeen:     time.Now(),
	}

	key := msg.InstanceName

	d.nodeLock.Lock()
	_, exists := d.discoveredNodes[key]
	d.discoveredNodes[key] = node
	d.nodeLock.Unlock()

	if !exists {
		d.logger.Info("discovered new Nomad node via broadcast",
			"instance", msg.InstanceName,
			"host", host,
			"serf_port", msg.SerfPort,
		)
	}
}

// cleanupLoop removes stale nodes periodically
func (d *BroadcastDiscovery) cleanupLoop() {
	defer d.wg.Done()

	ticker := time.NewTicker(NodeTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-d.shutdownCh:
			return
		case <-ticker.C:
			d.cleanupStaleNodes()
		}
	}
}

// cleanupStaleNodes removes nodes that haven't been seen recently
func (d *BroadcastDiscovery) cleanupStaleNodes() {
	d.nodeLock.Lock()
	defer d.nodeLock.Unlock()

	now := time.Now()
	for key, node := range d.discoveredNodes {
		if now.Sub(node.LastSeen) > NodeTimeout {
			delete(d.discoveredNodes, key)
			d.logger.Debug("removed stale node", "instance", node.InstanceName)
		}
	}
}

// JoinAddresses returns addresses of discovered nodes for Serf joining
func (d *BroadcastDiscovery) JoinAddresses() []string {
	d.nodeLock.RLock()
	defer d.nodeLock.RUnlock()

	var addrs []string
	now := time.Now()
	for _, node := range d.discoveredNodes {
		// Only return nodes seen in the last timeout period
		if now.Sub(node.LastSeen) < NodeTimeout {
			addr := fmt.Sprintf("%s:%d", node.Host, node.SerfPort)
			addrs = append(addrs, addr)
		}
	}
	return addrs
}

// HasDiscoveredNodes returns true if any nodes have been discovered
func (d *BroadcastDiscovery) HasDiscoveredNodes() bool {
	d.nodeLock.RLock()
	defer d.nodeLock.RUnlock()
	return len(d.discoveredNodes) > 0
}

// GetDiscoveredNodes returns all discovered nodes
func (d *BroadcastDiscovery) GetDiscoveredNodes() []*DiscoveredNode {
	d.nodeLock.RLock()
	defer d.nodeLock.RUnlock()

	nodes := make([]*DiscoveredNode, 0, len(d.discoveredNodes))
	for _, node := range d.discoveredNodes {
		nodes = append(nodes, node)
	}
	return nodes
}
