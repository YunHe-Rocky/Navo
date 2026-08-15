package network

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	endpointResolveAttempts  = 4
	endpointResolveTimeout   = 5 * time.Second
	endpointResolveRetryWait = 500 * time.Millisecond
)

type endpointLookup func(context.Context, string) ([]net.IP, error)

// ResolveEndpointIPs waits through transient cold-start DNS failures and
// returns canonical endpoint addresses without mutating network state.
func ResolveEndpointIPs(ctx context.Context, host string) ([]net.IP, error) {
	lookup := func(ctx context.Context, host string) ([]net.IP, error) {
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		result := make([]net.IP, 0, len(addresses))
		for _, address := range addresses {
			result = append(result, address.IP)
		}
		return result, nil
	}
	return resolveEndpointIPsWithRetry(
		ctx,
		host,
		endpointResolveAttempts,
		endpointResolveTimeout,
		endpointResolveRetryWait,
		lookup,
	)
}

func resolveEndpointIPsWithRetry(
	ctx context.Context,
	host string,
	attempts int,
	attemptTimeout time.Duration,
	retryWait time.Duration,
	lookup endpointLookup,
) ([]net.IP, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil, fmt.Errorf("endpoint host is empty")
	}
	if parsed := net.ParseIP(host); parsed != nil {
		return []net.IP{parsed}, nil
	}
	if attempts < 1 || lookup == nil {
		return nil, fmt.Errorf("endpoint resolver is unavailable")
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		attemptCtx := ctx
		cancel := func() {}
		if attemptTimeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, attemptTimeout)
		}
		addresses, err := lookup(attemptCtx, host)
		cancel()
		if err == nil {
			if canonical := canonicalEndpointIPs(addresses); len(canonical) > 0 {
				return canonical, nil
			}
			err = fmt.Errorf("resolver returned no usable addresses")
		}
		lastErr = err
		if attempt == attempts {
			break
		}
		wait := time.Duration(attempt) * retryWait
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("resolve endpoint %s: %w", host, ctx.Err())
		case <-timer.C:
		}
	}
	return nil, fmt.Errorf("resolve endpoint %s after %d attempts: %w", host, attempts, lastErr)
}

func canonicalEndpointIPs(addresses []net.IP) []net.IP {
	result := make([]net.IP, 0, len(addresses))
	seen := make(map[string]struct{}, len(addresses))
	for _, address := range addresses {
		if address == nil || address.IsUnspecified() || address.IsMulticast() {
			continue
		}
		canonical := address
		if ipv4 := address.To4(); ipv4 != nil {
			canonical = ipv4
		} else if ipv6 := address.To16(); ipv6 != nil {
			canonical = ipv6
		} else {
			continue
		}
		key := canonical.String()
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, canonical)
	}
	return result
}
