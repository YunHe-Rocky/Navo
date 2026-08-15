package service

import (
	"context"
	"time"
)

type UDPVerificationStatus string

const (
	UDPVerified    UDPVerificationStatus = "verified"
	UDPUnsupported UDPVerificationStatus = "unsupported"
	UDPFailed      UDPVerificationStatus = "failed"
)

type VerifyRequest struct {
	SessionID   string
	DirectIP    string
	DirectMode  bool
	ProxyPort   int
	TUNDNSIPv4  string
	UDPRequired bool
}

type VerifyResult struct {
	DNS          bool                        `json:"dns"`
	DNSLatency   time.Duration               `json:"-"`
	DNSLatencyMS int64                       `json:"dns_latency_ms"`
	TCP          bool                        `json:"tcp"`
	HTTPS        bool                        `json:"https"`
	DirectIP     string                      `json:"direct_ip"`
	ExitIP       string                      `json:"exit_ip"`
	ProxyExitIP  string                      `json:"proxy_exit_ip,omitempty"`
	Sites        map[string]SiteVerification `json:"sites,omitempty"`
	UDP          UDPVerificationStatus       `json:"udp"`
	UDPReason    string                      `json:"udp_reason,omitempty"`
	VerifiedAt   time.Time                   `json:"verified_at"`
}

type SiteVerification struct {
	DNS        bool `json:"dns"`
	TCP        bool `json:"tcp"`
	HTTPS      bool `json:"https"`
	StatusCode int  `json:"status_code,omitempty"`
}

type TUNDataPlaneVerifier interface {
	CaptureDirectIP(ctx context.Context) (string, error)
	Verify(ctx context.Context, request VerifyRequest) (VerifyResult, error)
}
