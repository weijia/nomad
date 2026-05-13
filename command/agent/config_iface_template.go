// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !android

package agent

import (
	"strings"

	"github.com/hashicorp/go-sockaddr/template"
)

// parseInterfaceTemplate parses a go-sockaddr template.
// On non-Android platforms, it uses the standard template parsing.
func parseInterfaceTemplate(tpl string) (string, error) {
	out, err := template.Parse(tpl)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
