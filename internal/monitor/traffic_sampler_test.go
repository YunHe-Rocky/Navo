package monitor

import (
	"testing"
	"time"
)

func TestTrafficSamplerCalculatesIndependentSeries(t *testing.T) {
	var sampler TrafficSampler
	start := time.Unix(100, 0)
	_ = sampler.Sample(start, 100, 200, 300, 400, true, true)
	sample := sampler.Sample(start.Add(2*time.Second), 300, 600, 900, 1200, true, true)
	if sample.LocalUploadBPS != 100 || sample.LocalDownloadBPS != 200 ||
		sample.ProxyUploadBPS != 300 || sample.ProxyDownloadBPS != 400 {
		t.Fatalf("sample = %#v", sample)
	}
	if sample.SourceState != "ready" {
		t.Fatalf("source state = %q", sample.SourceState)
	}
}

func TestTrafficSamplerHandlesResetAndSleep(t *testing.T) {
	var sampler TrafficSampler
	start := time.Unix(100, 0)
	_ = sampler.Sample(start, 1000, 1000, 1000, 1000, true, true)
	reset := sampler.Sample(start.Add(time.Second), 10, 20, 30, 40, true, true)
	if reset.LocalUploadBPS != 0 || reset.ProxyDownloadBPS != 0 {
		t.Fatalf("counter reset produced traffic spike: %#v", reset)
	}
	stale := sampler.Sample(start.Add(time.Minute), 100, 200, 300, 400, true, true)
	if stale.SourceState != "reset" || stale.LocalDownloadBPS != 0 {
		t.Fatalf("sleep interval produced traffic spike: %#v", stale)
	}
}
