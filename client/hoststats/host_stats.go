// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !android

package hoststats

import (
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

// Collect collects stats related to resource usage of a host
func (h *HostStatsCollector) Collect() error {
	h.hostStatsLock.Lock()
	defer h.hostStatsLock.Unlock()
	return h.collectLocked()
}

// collectLocked collects stats related to resource usage of the host but should
// be called with the lock held.
func (h *HostStatsCollector) collectLocked() error {
	hs := &HostStats{Timestamp: time.Now().UTC().UnixNano()}

	// Determine up-time
	uptime, err := host.Uptime()
	if err != nil {
		h.logger.Error("failed to collect upstime stats", "error", err)
		uptime = 0
	}
	hs.Uptime = uptime

	// Collect memory stats
	mstats, err := h.collectMemoryStats()
	if err != nil {
		h.logger.Error("failed to collect memory stats", "error", err)
		mstats = &MemoryStats{}
	}
	hs.Memory = mstats

	// Collect cpu stats
	cpus, ticks, err := h.collectCPUStats()
	if err != nil {
		h.logger.Error("failed to collect cpu stats", "error", err)
		cpus = []*CPUStats{}
		ticks = 0
	}
	hs.CPU = cpus
	hs.CPUTicksConsumed = ticks

	// Collect disk stats
	diskStats, err := h.collectDiskStats()
	if err != nil {
		h.logger.Error("failed to collect disk stats", "error", err)
		hs.DiskStats = []*DiskStats{}
	}
	hs.DiskStats = diskStats

	// Getting the disk stats for the allocation directory
	hs.AllocDirStats = h.getAllocDirDiskStats()

	// Collect devices stats
	hs.DeviceStats = h.collectDeviceGroupStats()

	// Update the collected status object.
	h.hostStats = hs

	return nil
}

func (h *HostStatsCollector) collectMemoryStats() (*MemoryStats, error) {
	memStats, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	return &MemoryStats{
		Total:     memStats.Total,
		Available: memStats.Available,
		Used:      memStats.Used,
		Free:      memStats.Free,
	}, nil
}

func (h *HostStatsCollector) collectDiskStats() ([]*DiskStats, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	var diskStats []*DiskStats
	for _, partition := range partitions {
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			if _, ok := h.badParts[partition.Mountpoint]; !ok {
				h.logger.Warn("failed to find disk usage for partition", "partition", partition.Mountpoint, "error", err)
				h.badParts[partition.Mountpoint] = struct{}{}
			}
			continue
		}

		diskStats = append(diskStats, h.toDiskStats(usage, &partition))
	}

	return diskStats, nil
}

func (h *HostStatsCollector) collectDeviceGroupStats() []*DeviceGroupStats {
	if h.deviceStatsCollector == nil {
		return []*DeviceGroupStats{}
	}

	return h.deviceStatsCollector()
}

func (h *HostStatsCollector) toDiskStats(usage *disk.UsageStat, partitionStat *disk.PartitionStat) *DiskStats {
	ds := DiskStats{
		Device:            usage.Path,
		Mountpoint:        usage.Path,
		Size:              usage.Total,
		Used:              usage.Used,
		Available:         usage.Free,
		UsedPercent:       usage.UsedPercent,
		InodesUsedPercent: usage.InodesUsedPercent,
	}

	if partitionStat != nil {
		ds.Device = partitionStat.Device
		ds.Mountpoint = partitionStat.Mountpoint
	}

	return &ds
}

func (h *HostStatsCollector) collectCPUStats() (cpus []*CPUStats, totalTicks float64, err error) {
	ticksConsumed := 0.0
	cpuStats, err := cpu.Times(true)
	if err != nil {
		return nil, 0.0, err
	}
	cs := make([]*CPUStats, len(cpuStats))
	for idx, cpuStat := range cpuStats {
		percentCalculator, ok := h.statsCalculator[cpuStat.CPU]
		if !ok {
			percentCalculator = NewHostCpuStatsCalculator()
			h.statsCalculator[cpuStat.CPU] = percentCalculator
		}
		idle, user, system, total := percentCalculator.Calculate(cpuStat)
		totalCompute := h.top.TotalCompute()
		ticks := (total / 100.0) * (float64(totalCompute) / float64(len(cpuStats)))
		cs[idx] = &CPUStats{
			CPU:           cpuStat.CPU,
			User:          user,
			System:        system,
			Idle:          idle,
			TotalPercent:  total,
			TotalTicks:    ticks,
		}
		ticksConsumed += ticks
	}

	return cs, ticksConsumed, nil
}

func (h *HostStatsCollector) getAllocDirDiskStats() *DiskStats {
	usage, err := disk.Usage(h.allocDir)
	if err != nil {
		h.logger.Error("failed to find disk usage of alloc", "alloc_dir", h.allocDir, "error", err)
		return &DiskStats{}
	}

	return &DiskStats{
		Device:            usage.Path,
		Mountpoint:        usage.Path,
		Size:              usage.Total,
		Used:              usage.Used,
		Available:         usage.Free,
		UsedPercent:       usage.UsedPercent,
		InodesUsedPercent: usage.InodesUsedPercent,
	}
}
