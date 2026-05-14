// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package hoststats

import (
	"math"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/shirou/gopsutil/v3/cpu"
)

// HostStats represents resource usage stats of the host running a Nomad client
type HostStats struct {
	Memory           *MemoryStats
	CPU              []*CPUStats
	DiskStats        []*DiskStats
	AllocDirStats    *DiskStats
	DeviceStats      []*DeviceGroupStats
	Uptime           uint64
	Timestamp        int64
	CPUTicksConsumed float64
}

// MemoryStats represents stats related to virtual memory usage
type MemoryStats struct {
	Total     uint64
	Available uint64
	Used      uint64
	Free      uint64
}

// CPUStats represents stats related to cpu usage
type CPUStats struct {
	CPU          string
	User         float64
	System       float64
	Idle         float64
	TotalPercent float64
	TotalTicks   float64
}

// DiskStats represents stats related to disk usage
type DiskStats struct {
	Device            string
	Mountpoint        string
	Size              uint64
	Used              uint64
	Available         uint64
	UsedPercent       float64
	InodesUsedPercent float64
}

// DeviceGroupStats represents stats related to device group
type DeviceGroupStats = device.DeviceGroupStats

// DeviceStatsCollector is used to retrieve all the latest statistics for all devices.
type DeviceStatsCollector func() []*DeviceGroupStats

// NodeStatsCollector is an interface which is used for the purposes of mocking
// the HostStatsCollector in the tests
type NodeStatsCollector interface {
	Collect() error
	Stats() *HostStats
}

// HostCpuStatsCalculator calculates cpu usage percentages
type HostCpuStatsCalculator struct {
	prevIdle   float64
	prevUser   float64
	prevSystem float64
	prevTotal  float64
	prevBusy   float64
}

// NewHostCpuStatsCalculator returns a HostCpuStatsCalculator
func NewHostCpuStatsCalculator() *HostCpuStatsCalculator {
	return &HostCpuStatsCalculator{}
}

// Calculate calculates the current cpu usage percentages
func (h *HostCpuStatsCalculator) Calculate(times cpu.TimesStat) (idle float64, user float64, system float64, total float64) {
	currentIdle := times.Idle
	currentUser := times.User
	currentSystem := times.System
	currentTotal := times.Total() // this is Idle + currentBusy
	currentBusy := times.User + times.System + times.Nice + times.Iowait + times.Irq +
		times.Softirq + times.Steal + times.Guest + times.GuestNice

	deltaTotal := currentTotal - h.prevTotal
	idle = ((currentIdle - h.prevIdle) / deltaTotal) * 100
	user = ((currentUser - h.prevUser) / deltaTotal) * 100
	system = ((currentSystem - h.prevSystem) / deltaTotal) * 100
	total = ((currentBusy - h.prevBusy) / deltaTotal) * 100

	// Protect against any invalid values
	if math.IsNaN(idle) || math.IsInf(idle, 0) || idle < 0.0 {
		idle = 100.0
	}
	if math.IsNaN(user) || math.IsInf(user, 0) || user < 0.0 {
		user = 0.0
	}
	if math.IsNaN(system) || math.IsInf(system, 0) || system < 0.0 {
		system = 0.0
	}
	if math.IsNaN(total) || math.IsInf(total, 0) || total < 0.0 {
		total = 0.0
	}

	h.prevIdle = currentIdle
	h.prevUser = currentUser
	h.prevSystem = currentSystem
	h.prevTotal = currentTotal
	h.prevBusy = currentBusy
	return
}

// HostStatsCollector collects host resource usage stats
type HostStatsCollector struct {
	top                  *numalib.Topology
	statsCalculator      map[string]*HostCpuStatsCalculator
	hostStats            *HostStats
	hostStatsLock        sync.RWMutex
	allocDir             string
	deviceStatsCollector DeviceStatsCollector

	// badParts is a set of partitions whose usage cannot be read; used to
	// squelch logspam.
	badParts map[string]struct{}

	logger hclog.Logger
}

// NewHostStatsCollector returns a HostStatsCollector. The allocDir is passed in
// so that we can present the disk related statistics for the mountpoint where
// the allocation directory lives
func NewHostStatsCollector(logger hclog.Logger, top *numalib.Topology, allocDir string, deviceStatsCollector DeviceStatsCollector) *HostStatsCollector {
	return &HostStatsCollector{
		logger:               logger.Named("host_stats"),
		top:                  top,
		statsCalculator:      make(map[string]*HostCpuStatsCalculator),
		allocDir:             allocDir,
		badParts:             make(map[string]struct{}),
		deviceStatsCollector: deviceStatsCollector,
	}
}

// Stats returns the stats
func (h *HostStatsCollector) Stats() *HostStats {
	h.hostStatsLock.RLock()
	defer h.hostStatsLock.RUnlock()
	return h.hostStats
}
