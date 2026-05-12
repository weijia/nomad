// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build android

package fingerprint

func initPlatformFingerprints(fps map[string]Factory) {
	// Android does not support cgroup or bridge fingerprinting
	// landlock is excluded because Android kernel lacks landlock syscall
}
