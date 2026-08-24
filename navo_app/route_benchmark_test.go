package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestRouteBenchmarkApplicationSwitchesBenchmarksAndRestores(t *testing.T) {
	var calls []string
	application := routeBenchmarkApplication{
		snapshot: func() (Dashboard, error) {
			return Dashboard{Runtime: RuntimeStatus{SelectedID: "route-a"}, Capture: CaptureStatus{CommittedMode: "off"}}, nil
		},
		selectOne: func(id string) error {
			calls = append(calls, "select:"+id)
			return nil
		},
		benchmark: func() (ProxyBenchmark, error) {
			calls = append(calls, "benchmark")
			return ProxyBenchmark{DownloadMbps: 12.5}, nil
		},
	}

	result, err := application.Run("route-b")
	if err != nil {
		t.Fatal(err)
	}
	if result.DownloadMbps != 12.5 {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := []string{"select:route-b", "benchmark", "select:route-a"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestRouteBenchmarkApplicationRejectsTemporarySwitchDuringCapture(t *testing.T) {
	selected := false
	application := routeBenchmarkApplication{
		snapshot: func() (Dashboard, error) {
			return Dashboard{Runtime: RuntimeStatus{SelectedID: "route-a"}, Capture: CaptureStatus{CommittedMode: "tun"}}, nil
		},
		selectOne: func(string) error { selected = true; return nil },
		benchmark: func() (ProxyBenchmark, error) { return ProxyBenchmark{}, nil },
	}

	_, err := application.Run("route-b")
	if err == nil || !strings.Contains(err.Error(), "接管期间") {
		t.Fatalf("expected capture rejection, got %v", err)
	}
	if selected {
		t.Fatal("route was mutated after capture rejection")
	}
}

func TestRouteBenchmarkApplicationReportsRestoreFailure(t *testing.T) {
	application := routeBenchmarkApplication{
		snapshot: func() (Dashboard, error) {
			return Dashboard{Runtime: RuntimeStatus{SelectedID: "route-a"}, Capture: CaptureStatus{CommittedMode: "off"}}, nil
		},
		selectOne: func(id string) error {
			if id == "route-a" {
				return errors.New("restore rejected")
			}
			return nil
		},
		benchmark: func() (ProxyBenchmark, error) { return ProxyBenchmark{}, nil },
	}

	_, err := application.Run("route-b")
	if err == nil || !strings.Contains(err.Error(), "恢复线路失败") {
		t.Fatalf("expected restore failure, got %v", err)
	}
}
