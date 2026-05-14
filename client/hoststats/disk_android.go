// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build android

package hoststats

import (
	"github.com/shirou/gopsutil/v3/disk"
)

// collectDiskStats collects disk stats on Android.
// Android doesn't have /proc/filesystems, so we use a simplified approach.
func (h *HostStatsCollector) collectDiskStats() ([]*DiskStats, error) {
	// On Android, just check the allocDir and common paths
	paths := []string{h.allocDir, "/data", "/sdcard", "/storage/emulated"}

	var diskStats []*DiskStats
	for _, path := range paths {
		usage, err := disk.Usage(path)
		if err != nil {
			// Path might not exist, skip silently
			continue
		}

		diskStats = append(diskStats, &DiskStats{
			Device:            path,
			Mountpoint:        path,
			Size:              usage.Total,
			Used:              usage.Used,
			Available:         usage.Free,
			UsedPercent:       usage.UsedPercent,
			InodesUsedPercent: usage.InodesUsedPercent,
		})
	}

	return diskStats, nil
}
