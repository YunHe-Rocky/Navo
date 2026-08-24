package service

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	proxyHTTPSVerifyFailed  = "PROXY_HTTPS_VERIFY_FAILED"
	proxyExitIPVerifyFailed = "PROXY_EXIT_IP_VERIFY_FAILED"
)

type RuntimeRoutingVerification struct {
	Verified    bool                        `json:"verified"`
	DirectIP    string                      `json:"direct_ip,omitempty"`
	ProxyExitIP string                      `json:"proxy_exit_ip,omitempty"`
	Sites       map[string]SiteVerification `json:"sites,omitempty"`
	VerifiedAt  time.Time                   `json:"verified_at"`
}

func validateRuntimeExitIdentity(mode, directIP, proxyExitIP string) error {
	directParsed := net.ParseIP(strings.TrimSpace(directIP))
	proxyParsed := net.ParseIP(strings.TrimSpace(proxyExitIP))
	if directParsed == nil || proxyParsed == nil {
		return proxyExitIdentityError(mode, directIP, proxyExitIP, "valid public IP evidence")
	}
	directIP, proxyExitIP = directParsed.String(), proxyParsed.String()
	if mode == runtimeModeDirect {
		if directIP != proxyExitIP {
			return proxyExitIdentityError(mode, directIP, proxyExitIP, "proxy exit equals direct exit")
		}
		return nil
	}
	if directIP == proxyExitIP {
		return proxyExitIdentityError(mode, directIP, proxyExitIP, "proxy exit differs from direct exit")
	}
	return nil
}

func proxyExitIdentityError(mode, directIP, proxyExitIP, expected string) error {
	return &captureTransitionError{
		code: proxyExitIPVerifyFailed,
		err: fmt.Errorf(
			"proxy exit identity verification failed: mode=%s expected=%s direct=%s proxy=%s",
			mode, expected, directIP, proxyExitIP,
		),
	}
}

func proxyRoutingVerificationError(resource, expected, actual string, cause error) error {
	if cause == nil {
		cause = errors.New("unexpected proxy response")
	}
	return &captureTransitionError{
		code: proxyHTTPSVerifyFailed,
		err: fmt.Errorf(
			"proxy routing verification failed: resource=%s expected=%s actual=%s: %w",
			resource, expected, actual, cause,
		),
	}
}
