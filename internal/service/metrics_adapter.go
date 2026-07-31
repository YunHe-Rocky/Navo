package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"navo/internal/coreadapter"
	"navo/internal/domain/core"
)

const maxMetricsConfigBytes = 2 << 20

func (s *Service) readCoreMetrics(
	ctx context.Context,
	coreID string,
	configPath string,
) (coreadapter.Metrics, error) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()

	if !s.metricsInitialized || s.metricsConfig != configPath || s.metricsCoreID != coreID {
		reader, reason, err := s.metricsReaderFromConfig(coreID, configPath)
		s.metricsInitialized = true
		if err != nil {
			s.metricsReader = nil
			s.metricsReason = reason
			return coreadapter.Metrics{}, err
		}
		s.metricsReader = reader
		s.metricsConfig = configPath
		s.metricsCoreID = coreID
		s.metricsReason = reason
	}
	if s.metricsReader == nil {
		if s.metricsReason == "" {
			s.metricsReason = "current core does not expose metrics"
		}
		return coreadapter.Metrics{}, fmt.Errorf("%s", s.metricsReason)
	}
	return s.metricsReader.Read(ctx)
}

func (s *Service) metricsReaderFromConfig(
	coreID string,
	configPath string,
) (coreadapter.MetricsReader, string, error) {
	coreType := core.Type(coreID)
	adapter, err := s.coreAdapters.Get(coreType)
	if err != nil {
		return nil, "core adapter is unavailable", err
	}
	if coreType == core.TypeXray {
		return nil, "Xray Stats API adapter is not enabled", nil
	}
	runtime, err := metricsRuntimeFromConfig(coreType, configPath)
	if err != nil {
		return nil, "metrics controller configuration is unavailable", err
	}
	reader := adapter.MetricsReader(runtime)
	if reader == nil {
		return nil, "current core does not implement MetricsReader", nil
	}
	return reader, "", nil
}

func metricsRuntimeFromConfig(
	coreType core.Type,
	configPath string,
) (coreadapter.RuntimeInfo, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return coreadapter.RuntimeInfo{}, fmt.Errorf("open metrics config: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return coreadapter.RuntimeInfo{}, fmt.Errorf("stat metrics config: %w", err)
	}
	if info.Size() > maxMetricsConfigBytes {
		return coreadapter.RuntimeInfo{}, fmt.Errorf("metrics config exceeds %d bytes", maxMetricsConfigBytes)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return coreadapter.RuntimeInfo{}, fmt.Errorf("read metrics config: %w", err)
	}

	var controller, secret string
	switch coreType {
	case core.TypeSingBox:
		var config struct {
			Experimental struct {
				ClashAPI struct {
					ExternalController string `json:"external_controller"`
					Secret             string `json:"secret"`
				} `json:"clash_api"`
			} `json:"experimental"`
		}
		if err := json.Unmarshal(data, &config); err != nil {
			return coreadapter.RuntimeInfo{}, fmt.Errorf("decode sing-box metrics config: %w", err)
		}
		controller = config.Experimental.ClashAPI.ExternalController
		secret = config.Experimental.ClashAPI.Secret
	case core.TypeMihomo:
		var config struct {
			ExternalController string `yaml:"external-controller"`
			Secret             string `yaml:"secret"`
		}
		if err := yaml.Unmarshal(data, &config); err != nil {
			return coreadapter.RuntimeInfo{}, fmt.Errorf("decode Mihomo metrics config: %w", err)
		}
		controller, secret = config.ExternalController, config.Secret
	default:
		return coreadapter.RuntimeInfo{}, fmt.Errorf("metrics unsupported for core %s", coreType)
	}
	controller = strings.TrimSpace(controller)
	if controller == "" {
		return coreadapter.RuntimeInfo{}, fmt.Errorf("metrics controller is not configured")
	}
	if !strings.Contains(controller, "://") {
		controller = "http://" + controller
	}
	parsed, err := url.Parse(controller)
	if err != nil || parsed.Host == "" {
		return coreadapter.RuntimeInfo{}, fmt.Errorf("invalid metrics controller address")
	}
	if parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost" {
		return coreadapter.RuntimeInfo{}, fmt.Errorf("metrics controller must be loopback-only")
	}
	return coreadapter.RuntimeInfo{
		ControllerURL: controller, ControllerSecret: secret,
	}, nil
}
