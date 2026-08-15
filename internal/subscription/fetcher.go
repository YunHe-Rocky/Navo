// Package subscription handles airport subscription fetching, parsing, and node management.
package subscription

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Fetcher downloads subscription content from a URL with safety limits.
type Fetcher struct {
	secureClient   *http.Client
	insecureClient *http.Client
	maxSize        int64
}

// NewFetcher creates a new Fetcher with safe defaults.
func NewFetcher() *Fetcher {
	return &Fetcher{
		secureClient:   newSafeHTTPClient(false, net.DefaultResolver),
		insecureClient: newSafeHTTPClient(true, net.DefaultResolver),
		maxSize:        10 * 1024 * 1024, // 10MB
	}
}

func newHTTPClient(skipTLSVerify bool) *http.Client {
	return newClientWithDialer(skipTLSVerify, (&net.Dialer{Timeout: 10 * time.Second}).DialContext)
}

type ipResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type dialContextFunc func(context.Context, string, string) (net.Conn, error)

func newSafeHTTPClient(skipTLSVerify bool, resolver ipResolver) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return newClientWithDialer(
		skipTLSVerify,
		newValidatedDialer(resolver, dialer.DialContext),
	)
}

func newValidatedDialer(
	resolver ipResolver,
	dial dialContextFunc,
) dialContextFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid destination: %w", err)
		}
		var addresses []net.IPAddr
		if parsed := net.ParseIP(host); parsed != nil {
			addresses = []net.IPAddr{{IP: parsed}}
		} else {
			addresses, err = resolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("resolve subscription host: %w", err)
			}
		}
		if len(addresses) == 0 {
			return nil, fmt.Errorf("subscription host resolved to no addresses")
		}
		for _, candidate := range addresses {
			if isForbiddenIP(candidate.IP) {
				return nil, fmt.Errorf("subscription host resolved to forbidden address")
			}
		}

		// Try only the already-validated snapshot. Never hand the hostname back to
		// a resolver where DNS rebinding could change the destination.
		dialErrors := make([]error, 0, len(addresses))
		for _, candidate := range addresses {
			connection, dialErr := dial(
				ctx,
				network,
				net.JoinHostPort(candidate.IP.String(), port),
			)
			if dialErr == nil {
				return connection, nil
			}
			dialErrors = append(dialErrors, dialErr)
			if ctx.Err() != nil {
				break
			}
		}
		return nil, fmt.Errorf(
			"dial validated subscription addresses: %w",
			errors.Join(dialErrors...),
		)
	}
}

func newClientWithDialer(
	skipTLSVerify bool,
	dialContext dialContextFunc,
) *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext:       dialContext,
			DisableKeepAlives: true,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				// This is used only for an explicitly opted-in subscription.
				InsecureSkipVerify: skipTLSVerify, //nolint:gosec
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects (max 3)")
			}
			if err := validateURL(req.URL.String()); err != nil {
				return fmt.Errorf("unsafe redirect: %w", err)
			}
			return nil
		},
	}
}

// FetchOptions controls compatibility for one subscription request.
type FetchOptions struct {
	SkipTLSVerify bool
}

// Fetch downloads a subscription from the given URL.
// Returns the raw content bytes.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	return f.FetchWithOptions(ctx, rawURL, FetchOptions{})
}

// FetchWithOptions downloads a subscription with provider-scoped options.
func (f *Fetcher) FetchWithOptions(
	ctx context.Context,
	rawURL string,
	options FetchOptions,
) ([]byte, error) {
	if err := validateURL(rawURL); err != nil {
		return nil, fmt.Errorf("invalid subscription URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Navo/1.0")

	client := f.secureClient
	if options.SkipTLSVerify {
		client = f.insecureClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", withoutRequestURL(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subscription server returned %d", resp.StatusCode)
	}

	// Limit read to maxSize
	limited := io.LimitReader(resp.Body, f.maxSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if int64(len(data)) > f.maxSize {
		return nil, fmt.Errorf("subscription too large (max %d bytes)", f.maxSize)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("empty subscription response")
	}

	return data, nil
}

func withoutRequestURL(err error) error {
	for {
		var urlError *url.Error
		if !errors.As(err, &urlError) || urlError.Err == nil {
			return err
		}
		err = urlError.Err
	}
}

// validateURL checks that a subscription URL is safe to fetch.
func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow HTTPS
	if u.Scheme != "https" {
		return fmt.Errorf("only HTTPS allowed, got %q", u.Scheme)
	}

	// SSRF protection: forbid private/dangerous hosts
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return fmt.Errorf("empty host")
	}

	dangerousHosts := []string{"127.0.0.1", "localhost", "0.0.0.0", "::1"}
	for _, d := range dangerousHosts {
		if host == d {
			return fmt.Errorf("host %q is forbidden", host)
		}
	}

	// Forbid private IP ranges
	ip := net.ParseIP(host)
	if ip != nil && isForbiddenIP(ip) {
		return fmt.Errorf("forbidden IP %q (SSRF)", host)
	}

	return nil
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

func isForbiddenIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
		return true
	}
	for _, cidr := range []string{
		"0.0.0.0/8", "100.64.0.0/10", "192.0.2.0/24", "198.51.100.0/24",
		"203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4", "2001:db8::/32",
	} {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return true
		}
	}
	return false
}
