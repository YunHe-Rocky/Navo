package monitor

import (
	"sync"
	"time"
)

type TrafficSample struct {
	Timestamp          time.Time `json:"timestamp"`
	LocalUploadBPS     uint64    `json:"local_upload_bps"`
	LocalDownloadBPS   uint64    `json:"local_download_bps"`
	ProxyUploadBPS     uint64    `json:"proxy_upload_bps"`
	ProxyDownloadBPS   uint64    `json:"proxy_download_bps"`
	LocalUploadTotal   uint64    `json:"local_upload_total"`
	LocalDownloadTotal uint64    `json:"local_download_total"`
	ProxyUploadTotal   uint64    `json:"proxy_upload_total"`
	ProxyDownloadTotal uint64    `json:"proxy_download_total"`
	SourceState        string    `json:"source_state"`
}

type TrafficSampler struct {
	mu                     sync.Mutex
	previous               *TrafficSample
	previousLocalAvailable bool
	previousProxyAvailable bool
}

func (s *TrafficSampler) Sample(
	now time.Time,
	localUpload, localDownload, proxyUpload, proxyDownload uint64,
	localAvailable, proxyAvailable bool,
) TrafficSample {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := TrafficSample{
		Timestamp:          now.UTC(),
		LocalUploadTotal:   localUpload,
		LocalDownloadTotal: localDownload,
		ProxyUploadTotal:   proxyUpload,
		ProxyDownloadTotal: proxyDownload,
		SourceState:        trafficSourceState(localAvailable, proxyAvailable),
	}
	previous := s.previous
	previousLocalAvailable := s.previousLocalAvailable
	previousProxyAvailable := s.previousProxyAvailable
	s.previous = &current
	s.previousLocalAvailable = localAvailable
	s.previousProxyAvailable = proxyAvailable
	if previous == nil {
		return current
	}
	elapsed := now.Sub(previous.Timestamp)
	if elapsed <= 0 || elapsed > 30*time.Second {
		current.SourceState = "reset"
		return current
	}
	seconds := elapsed.Seconds()
	if localAvailable && previousLocalAvailable {
		current.LocalUploadBPS = counterRate(localUpload, previous.LocalUploadTotal, seconds)
		current.LocalDownloadBPS = counterRate(localDownload, previous.LocalDownloadTotal, seconds)
	}
	if proxyAvailable && previousProxyAvailable {
		current.ProxyUploadBPS = counterRate(proxyUpload, previous.ProxyUploadTotal, seconds)
		current.ProxyDownloadBPS = counterRate(proxyDownload, previous.ProxyDownloadTotal, seconds)
	}
	return current
}

func counterRate(current, previous uint64, seconds float64) uint64 {
	if current < previous || seconds <= 0 {
		return 0
	}
	return uint64(float64(current-previous) / seconds)
}

func trafficSourceState(localAvailable, proxyAvailable bool) string {
	switch {
	case localAvailable && proxyAvailable:
		return "ready"
	case localAvailable || proxyAvailable:
		return "partial"
	default:
		return "unavailable"
	}
}
