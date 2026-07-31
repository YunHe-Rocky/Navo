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
		secureClient:   newHTTPClient(false),
		insecureClient: newHTTPClient(true),
		maxSize:        10 * 1024 * 1024, // 10MB
	}
}

func newHTTPClient(skipTLSVerify bool) *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext:       (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
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
	if ip != nil && isPrivateIP(ip) {
		return fmt.Errorf("private IP %q is forbidden (SSRF)", host)
	}

	return nil
}

func isPrivateIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		// 10.0.0.0/8
		if ip4[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
		// 169.254.0.0/16 (link-local)
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}
	return false
}
