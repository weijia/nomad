// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !android

package config

import "github.com/shoenig/go-landlock"

// validateLandlockPath validates a path for landlock filesystem isolation.
func validateLandlockPath(p string) error {
	_, err := landlock.ParsePath(p)
	return err
}
