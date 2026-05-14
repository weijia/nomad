// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !android

package agent

// applyAndroidDefaults is a no-op on non-Android platforms.
func applyAndroidDefaults(cfg *Config) {
	// No-op on non-Android platforms
}
