// Package monitor provides active and passive network monitoring.
package monitor

import (
	"context"
	"fmt"
	"net"
	"time"
)

// ProbeResult holds the results of a single probe against an outbound.
type ProbeResult struct {
	OutboundID string        `json:"outbound_id"`
	Latency    time.Duration `json:"latency"`
	DNSTime    time.Duration `json:"dns_time"`
	Healthy    bool          `json:"healthy"`
	Error      string        `json:"error,omitempty"`
	CheckedAt  time.Time     `json:"checked_at"`
}

// Prober performs active network tests against outbounds.
type Prober struct {
	timeout time.Duration
}

// NewProber creates a new Prober with a default 3s timeout.
func NewProber() *Prober {
	return &Prober{timeout: 3 * time.Second}
}

// ProbeTCP measures TCP connection latency to an outbound's server:port.
func (p *Prober) ProbeTCP(ctx context.Context, outboundID, server string, port int) *ProbeResult {
	result := &ProbeResult{
		OutboundID: outboundID,
		CheckedAt:  time.Now(),
	}

	addr := net.JoinHostPort(server, fmt.Sprintf("%d", port))
	start := time.Now()

	dialer := &net.Dialer{Timeout: p.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	conn.Close()

	result.Latency = time.Since(start)
	result.Healthy = true
	return result
}

// ProbeDNS measures DNS resolution time for a hostname using a specific resolver.
func (p *Prober) ProbeDNS(ctx context.Context, hostname string) *ProbeResult {
	result := &ProbeResult{
		CheckedAt: time.Now(),
	}

	start := time.Now()
	_, err := net.DefaultResolver.LookupHost(ctx, hostname)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.DNSTime = time.Since(start)
	result.Healthy = true
	return result
}

// ProbeHTTPS measures TLS handshake latency.
func (p *Prober) ProbeHTTPS(ctx context.Context, outboundID, server string, port int) *ProbeResult {
	result := &ProbeResult{
		OutboundID: outboundID,
		CheckedAt:  time.Now(),
	}

	addr := net.JoinHostPort(server, fmt.Sprintf("%d", port))
	start := time.Now()

	d := &net.Dialer{Timeout: p.timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	conn.Close()

	result.Latency = time.Since(start)
	result.Healthy = true
	return result
}
