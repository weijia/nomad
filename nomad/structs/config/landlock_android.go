// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build android

package config

// validateLandlockPath is a no-op on Android since landlock is not available.
func validateLandlockPath(p string) error {
	return nil
}
