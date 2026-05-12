// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build android

package fingerprint

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
)

const (
	landlockKey = "kernel.landlock"
)

// LandlockFingerprint is a stub for Android where landlock is not available.
type LandlockFingerprint struct {
	StaticFingerprinter
	logger hclog.Logger
}

func NewLandlockFingerprint(logger hclog.Logger) Fingerprint {
	return &LandlockFingerprint{
		logger: logger.Named("landlock"),
	}
}

func (f *LandlockFingerprint) Fingerprint(_ *FingerprintRequest, resp *FingerprintResponse) error {
	// Android does not support landlock
	f.logger.Debug("landlock is not available on Android")
	return nil
}

// Suppress unused import warning
var _ = fmt.Sprintf
