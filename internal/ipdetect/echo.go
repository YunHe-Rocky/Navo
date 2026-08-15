// Package ipdetect provides IP detection and GeoIP lookup via IP echo services.
package ipdetect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// IPResult holds the result of an IP check for a specific outbound.
type IPResult struct {
	OutboundID string    `json:"outbound_id"`
	IP         string    `json:"ip"`
	Country    string    `json:"country,omitempty"`
	City       string    `json:"city,omitempty"`
	ASN        string    `json:"asn,omitempty"`
	ISP        string    `json:"isp,omitempty"`
	Network    string    `json:"network,omitempty"`
	Provider   string    `json:"provider,omitempty"`
	Mobile     bool      `json:"mobile,omitempty"`
	Proxy      bool      `json:"proxy,omitempty"`
	Hosting    bool      `json:"hosting,omitempty"`
	Error      string    `json:"error,omitempty"`
	CheckedAt  time.Time `json:"checked_at"`
}

// DualIPResult holds both source (direct) and proxy IP results.
type DualIPResult struct {
	Source *IPResult `json:"source"`
	Proxy  *IPResult `json:"proxy"`
}

// Detector checks the current outbound IP via echo services.
type Detector struct {
	client      *http.Client
	cacheTTL    time.Duration
	endpoints   []echoEndpoint
	geoEndpoint string
	mu          sync.RWMutex
	cache       map[string]*cachedResult
}

type echoEndpoint struct {
	name string
	url  string
	json bool
}

type cachedResult struct {
	result    IPResult
	expiresAt time.Time
}

// geoResponse is the response from ip-api.com (free tier).
type geoResponse struct {
	Success    bool   `json:"success"`
	Country    string `json:"country"`
	City       string `json:"city"`
	Type       string `json:"type"`
	Connection struct {
		ASN  int    `json:"asn"`
		ISP  string `json:"isp"`
		Org  string `json:"org"`
		Type string `json:"type"`
	} `json:"connection"`
	Security struct {
		Proxy   bool `json:"proxy"`
		VPN     bool `json:"vpn"`
		Hosting bool `json:"hosting"`
	} `json:"security"`
}

// NewDetector creates a new IP Detector that connects directly (no proxy).
func NewDetector() *Detector {
	return newDetector(detectorTransport(nil))
}

// NewDetectorWithProxy checks the egress IP through Navo's local mixed proxy.
func NewDetectorWithProxy(proxyAddress string) *Detector {
	proxyURL, err := url.Parse(proxyAddress)
	if err != nil {
		return newDetector(detectorTransport(nil))
	}
	return newDetector(detectorTransport(proxyURL))
}

func newDetector(transport http.RoundTripper) *Detector {
	if transport == nil {
		transport = detectorTransport(nil)
	}
	return &Detector{
		client: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
		cacheTTL: 5 * time.Minute,
		endpoints: []echoEndpoint{
			{name: "amazon", url: "https://checkip.amazonaws.com"},
			{name: "ipify", url: "https://api.ipify.org?format=json", json: true},
			{name: "icanhazip", url: "https://icanhazip.com"},
			{name: "ifconfig.me", url: "https://ifconfig.me/ip"},
		},
		geoEndpoint: "https://ipwho.is/%s",
		cache:       make(map[string]*cachedResult),
	}
}

func detectorTransport(proxyURL *url.URL) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	} else {
		transport.Proxy = nil
	}
	// Each detector owns an independent pool. Keep HTTP/2 enabled because several
	// IP providers negotiate h2 and return binary frames that HTTP/1.x cannot parse.
	transport.ForceAttemptHTTP2 = true
	return transport
}

// Check detects the IP used by the current connection and fills in geo data.
func (d *Detector) Check(ctx context.Context, outboundID string) (*IPResult, error) {
	// Check cache first
	d.mu.RLock()
	if cached, ok := d.cache[outboundID]; ok && time.Now().Before(cached.expiresAt) {
		d.mu.RUnlock()
		result := cached.result
		return &result, nil
	}
	d.mu.RUnlock()

	attempts := 0
	for _, endpoint := range d.endpoints {
		if ctx.Err() != nil {
			break
		}
		attempts++
		requestCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
		ip, err := d.checkEndpoint(requestCtx, endpoint)
		cancel()
		if err != nil {
			continue
		}
		result := &IPResult{
			OutboundID: outboundID,
			IP:         ip,
			Provider:   endpoint.name,
			CheckedAt:  time.Now(),
		}

		// Geo lookup (best-effort, non-blocking for core result)
		geoCtx, geoCancel := context.WithTimeout(ctx, 2500*time.Millisecond)
		d.fillGeo(geoCtx, result)
		geoCancel()

		d.mu.Lock()
		d.cache[outboundID] = &cachedResult{
			result:    *result,
			expiresAt: time.Now().Add(d.cacheTTL),
		}
		d.mu.Unlock()
		return result, nil
	}
	return nil, fmt.Errorf("IP check unavailable: %d providers failed", attempts)
}

// CheckFresh bypasses the bounded cache for user-triggered and mode-sensitive checks.
func (d *Detector) CheckFresh(ctx context.Context, outboundID string) (*IPResult, error) {
	d.mu.Lock()
	delete(d.cache, outboundID)
	d.mu.Unlock()
	// A capture-mode transition may leave idle sockets bound to the previous
	// route. Active requests are unaffected; the next probe gets a fresh socket.
	d.client.CloseIdleConnections()
	return d.Check(ctx, outboundID)
}

// fillGeo performs a best-effort geo lookup for the IP.
func (d *Detector) fillGeo(ctx context.Context, result *IPResult) {
	if result == nil || result.IP == "" {
		return
	}
	geoURL := fmt.Sprintf(d.geoEndpoint, result.IP)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geoURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "Navo/1.0")
	resp, err := d.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	limited := io.LimitReader(resp.Body, 4096)
	body, err := io.ReadAll(limited)
	if err != nil {
		return
	}
	var geo geoResponse
	if err := json.Unmarshal(body, &geo); err != nil {
		return
	}
	if !geo.Success {
		return
	}
	result.Country = geo.Country
	result.City = geo.City
	if geo.Connection.ASN > 0 {
		result.ASN = fmt.Sprintf("AS%d", geo.Connection.ASN)
	}
	result.ISP = geo.Connection.ISP
	result.Network = geo.Connection.Org
	connectionType := strings.ToLower(geo.Connection.Type)
	result.Mobile = strings.Contains(connectionType, "mobile")
	result.Proxy = geo.Security.Proxy || geo.Security.VPN || strings.Contains(connectionType, "vpn")
	result.Hosting = geo.Security.Hosting || strings.Contains(connectionType, "hosting")
}

func (d *Detector) checkEndpoint(
	ctx context.Context,
	endpoint echoEndpoint,
) (string, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		endpoint.url,
		nil,
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Navo/1.0")
	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, 1025)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if len(body) > 1024 {
		return "", fmt.Errorf("IP response is too large")
	}
	ip := strings.TrimSpace(string(body))
	if endpoint.json {
		var payload struct {
			IP string `json:"ip"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", err
		}
		ip = strings.TrimSpace(payload.IP)
	}
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("provider returned an invalid IP address")
	}
	return ip, nil
}

// CheckAll detects IPs for multiple outbound IDs.
func (d *Detector) CheckAll(ctx context.Context, outboundIDs []string) []IPResult {
	results := make([]IPResult, len(outboundIDs))
	var wg sync.WaitGroup

	for i, id := range outboundIDs {
		wg.Add(1)
		go func(idx int, obID string) {
			defer wg.Done()
			res, err := d.Check(ctx, obID)
			if err != nil {
				results[idx] = IPResult{
					OutboundID: obID,
					Error:      err.Error(),
					CheckedAt:  time.Now(),
				}
			} else {
				results[idx] = *res
			}
		}(i, id)
	}

	wg.Wait()
	return results
}

// ClearCache removes all cached IP results.
func (d *Detector) ClearCache() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache = make(map[string]*cachedResult)
}
