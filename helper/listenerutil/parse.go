// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !android

package listenerutil

import "github.com/hashicorp/go-secure-stdlib/listenerutil"

// ParseAddressTemplate parses an address template.
// On non-Android platforms, it uses the standard listenerutil.ParseSingleIPTemplate.
func ParseAddressTemplate(addr string) (string, error) {
	return listenerutil.ParseSingleIPTemplate(addr)
}
