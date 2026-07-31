// Package parser provides subscription content parsers for various proxy protocols.
// Each protocol has its own parser file; no single "ParseEverything()" function.
package parser

import (
	"navo/internal/compiler"
)

// Result holds parsed node information from a subscription.
type Result struct {
	Outbounds []compiler.Outbound
	Errors    []string
}

// Parser is the interface for subscription content parsers.
type Parser interface {
	// Parse extracts nodes from raw subscription content.
	Parse(raw []byte) (*Result, error)

	// Supports returns true if this parser can handle the given content.
	Supports(raw []byte) bool
}
