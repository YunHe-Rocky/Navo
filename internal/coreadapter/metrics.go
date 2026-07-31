package coreadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxMetricsResponseBytes = 8 << 20

type clashMetricsReader struct {
	endpoint string
	secret   string
	client   *http.Client
}

func newClashMetricsReader(runtime RuntimeInfo) MetricsReader {
	baseURL := strings.TrimRight(strings.TrimSpace(runtime.ControllerURL), "/")
	if baseURL == "" {
		return nil
	}
	return &clashMetricsReader{
		endpoint: baseURL + "/connections",
		secret:   runtime.ControllerSecret,
		client:   &http.Client{Timeout: 1500 * time.Millisecond},
	}
}

func (r *clashMetricsReader) Read(ctx context.Context) (Metrics, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, r.endpoint, nil)
	if err != nil {
		return Metrics{}, fmt.Errorf("create metrics request: %w", err)
	}
	if r.secret != "" {
		request.Header.Set("Authorization", "Bearer "+r.secret)
	}
	response, err := r.client.Do(request)
	if err != nil {
		return Metrics{}, fmt.Errorf("request core metrics: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Metrics{}, fmt.Errorf("core metrics returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxMetricsResponseBytes+1))
	if err != nil {
		return Metrics{}, fmt.Errorf("read core metrics: %w", err)
	}
	if len(body) > maxMetricsResponseBytes {
		return Metrics{}, fmt.Errorf("core metrics response exceeds %d bytes", maxMetricsResponseBytes)
	}
	var payload struct {
		UploadTotal   uint64            `json:"uploadTotal"`
		DownloadTotal uint64            `json:"downloadTotal"`
		Connections   []json.RawMessage `json:"connections"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return Metrics{}, fmt.Errorf("decode core metrics: %w", err)
	}
	return Metrics{
		UploadBytes:   payload.UploadTotal,
		DownloadBytes: payload.DownloadTotal,
		Connections:   len(payload.Connections),
	}, nil
}
