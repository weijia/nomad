// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !android

package agent

import (
	"errors"
	"strings"

	"github.com/hashicorp/go-sockaddr/template"
)

// parseMultipleIPTemplate parses multiple IP addresses from a template.
// On non-Android platforms, it uses the standard template parsing.
func parseMultipleIPTemplate(ipTmpl string) ([]string, error) {
	out, err := template.Parse(ipTmpl)
	if err != nil {
		return []string{}, err
	}

	ips := strings.Split(out, " ")
	if len(ips) == 0 {
		return []string{}, errors.New("no addresses found")
	}

	return deduplicateAddrs(ips), nil
}
