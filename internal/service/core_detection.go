package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"navo/internal/coreadapter"
	"navo/internal/domain/capture"
	"navo/internal/domain/core"
)

type coreDetection struct {
	ID             string
	Name           string
	Version        string
	Installed      bool
	CaptureModes   []string
	SystemProxy    bool
	TUN            bool
	Controller     bool
	Metrics        bool
	DetectionError string
}

func (s *Service) detectCores() []coreDetection {
	s.coreDetectOnce.Do(func() {
		specs := []struct {
			id, name, path string
		}{
			{id: "sing-box", name: "sing-box", path: s.cfg.SingBoxPath},
			{id: "mihomo", name: "Mihomo", path: s.cfg.MihomoPath},
			{id: "xray", name: "Xray-core", path: s.cfg.XrayPath},
		}
		results := make([]coreDetection, len(specs))
		var wait sync.WaitGroup
		for index, spec := range specs {
			index, spec := index, spec
			wait.Add(1)
			go func() {
				defer wait.Done()
				result := coreDetection{ID: spec.id, Name: spec.name, Installed: fileExists(spec.path)}
				adapter, err := s.coreAdapters.Get(core.Type(spec.id))
				if err != nil {
					result.DetectionError = err.Error()
					results[index] = result
					return
				}
				capabilities := adapter.Capabilities(coreadapter.Version{})
				for _, mode := range []capture.Mode{capture.ModeOff, capture.ModeSystemProxy, capture.ModeTUN} {
					if capabilities.CaptureModes[mode] {
						result.CaptureModes = append(result.CaptureModes, mode.String())
					}
				}
				result.SystemProxy = capabilities.CaptureModes[capture.ModeSystemProxy]
				result.TUN = capabilities.CaptureModes[capture.ModeTUN]
				result.Controller = capabilities.Controller
				result.Metrics = capabilities.Metrics
				if result.Installed {
					ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
					version, versionErr := adapter.DetectVersion(ctx, spec.path)
					cancel()
					if versionErr != nil {
						result.DetectionError = versionErr.Error()
					} else {
						result.Version = strings.TrimSpace(version.Raw)
					}
				}
				results[index] = result
			}()
		}
		wait.Wait()
		s.coreDetections = results
	})
	return append([]coreDetection(nil), s.coreDetections...)
}

func (s *Service) coreSupportsCapture(coreID string, mode capture.Mode) bool {
	adapter, err := s.coreAdapters.Get(core.Type(coreID))
	if err != nil {
		return false
	}
	return adapter.Capabilities(coreadapter.Version{}).CaptureModes[mode]
}
