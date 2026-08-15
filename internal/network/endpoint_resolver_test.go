package network

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestResolveEndpointIPsWithRetryRecoversFromColdStartDNS(t *testing.T) {
	attempts := 0
	addresses, err := resolveEndpointIPsWithRetry(
		context.Background(),
		"proxy.example",
		4,
		50*time.Millisecond,
		0,
		func(context.Context, string) ([]net.IP, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("network stack is not ready")
			}
			return []net.IP{net.ParseIP("203.0.113.7"), net.ParseIP("203.0.113.7")}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 || len(addresses) != 1 || addresses[0].String() != "203.0.113.7" {
		t.Fatalf("addresses=%v attempts=%d", addresses, attempts)
	}
}

func TestResolveEndpointIPsWithRetryHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := resolveEndpointIPsWithRetry(
		ctx,
		"proxy.example",
		4,
		time.Second,
		time.Second,
		func(ctx context.Context, _ string) ([]net.IP, error) {
			return nil, ctx.Err()
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestResolveEndpointIPsWithRetryAcceptsLiteralWithoutLookup(t *testing.T) {
	called := false
	addresses, err := resolveEndpointIPsWithRetry(
		context.Background(),
		"192.0.2.8",
		1,
		time.Second,
		0,
		func(context.Context, string) ([]net.IP, error) {
			called = true
			return nil, nil
		},
	)
	if err != nil || called || len(addresses) != 1 || addresses[0].String() != "192.0.2.8" {
		t.Fatalf("addresses=%v called=%t err=%v", addresses, called, err)
	}
}
