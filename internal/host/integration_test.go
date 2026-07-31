//go:build integration

package host

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

func setBinaryPath(t *testing.T) {
	t.Setenv("NAVO_SINGBOX_PATH", "../../third_party/sing-box/sing-box.exe")
}

func TestSingBox_RealStartStop(t *testing.T) {
	setBinaryPath(t)
	binaryPath, err := FindBinary()
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	configPath := "../../configs/test_local.json"
	host := NewSingBoxHost(binaryPath, "")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	t.Log("Starting sing-box...")
	pid, err := host.Start(ctx, configPath)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	t.Logf("sing-box started with PID=%d", pid)

	// Verify process running
	status := host.Status()
	if status.State != HostStateRunning {
		t.Errorf("state = %s, want %s", status.State, HostStateRunning)
	}

	// Health check
	result := host.HealthCheck(ctx)
	if !result.ProcessOK {
		t.Error("ProcessOK should be true")
	}

	// Verify port is listening
	conn, err := net.DialTimeout("tcp", "127.0.0.1:2080", 2*time.Second)
	if err != nil {
		t.Errorf("port 2080 not listening: %v", err)
	} else {
		conn.Close()
		t.Log("port 2080 confirmed listening")
	}

	// Get logs
	logs, err := host.GetLogs(10)
	if err != nil {
		t.Logf("GetLogs error: %v", err)
	} else {
		t.Logf("Recent logs: %v", logs)
	}

	// Stop
	t.Log("Stopping sing-box...")
	if err := host.Stop(ctx, false, 5*time.Second); err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	status = host.Status()
	if status.State != HostStateStopped {
		t.Errorf("state after stop = %s, want %s", status.State, HostStateStopped)
	}

	t.Log("sing-box start/stop cycle completed successfully")
}

func TestSingBox_FiveCycles(t *testing.T) {
	setBinaryPath(t)
	binaryPath, err := FindBinary()
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	configPath := "../../configs/test_local.json"
	host := NewSingBoxHost(binaryPath, "")

	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		pid, err := host.Start(ctx, configPath)
		if err != nil {
			cancel()
			t.Fatalf("cycle %d start failed: %v", i+1, err)
		}

		conn, err := net.DialTimeout("tcp", "127.0.0.1:2080", 1*time.Second)
		if err != nil {
			cancel()
			t.Fatalf("cycle %d port check failed: %v", i+1, err)
		}
		conn.Close()

		if err := host.Stop(ctx, false, 3*time.Second); err != nil {
			cancel()
			t.Fatalf("cycle %d stop failed: %v", i+1, err)
		}
		cancel()
		time.Sleep(200 * time.Millisecond)

		t.Logf("cycle %d: PID=%d ✓", i+1, pid)
	}

	if isPortInUse(2080) {
		t.Error("port 2080 should be free after 5 stop cycles")
	}
	t.Log("5 start/stop cycles completed with no zombie processes")
}

func TestSingBox_ValidateConfig(t *testing.T) {
	setBinaryPath(t)
	binaryPath, err := FindBinary()
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	host := NewSingBoxHost(binaryPath, "")

	if err := host.ValidateConfig(context.Background(), "../../configs/test_local.json"); err != nil {
		t.Errorf("valid config should pass validation: %v", err)
	}

	err = host.ValidateConfig(context.Background(), "../../configs/nonexistent.json")
	if err == nil {
		t.Error("expected error for non-existent config")
	}
}

func TestSingBox_BinaryValidation(t *testing.T) {
	setBinaryPath(t)
	binaryPath, err := FindBinary()
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	if err := ValidateBinary(binaryPath); err != nil {
		t.Errorf("binary validation failed: %v", err)
	}
}

func TestHealthCheck_RealCore(t *testing.T) {
	setBinaryPath(t)
	binaryPath, err := FindBinary()
	if err != nil {
		t.Skip("sing-box binary not found:", err)
	}

	host := NewSingBoxHost(binaryPath, "")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	host.Start(ctx, "../../configs/test_local.json")

	result := host.HealthCheck(ctx)
	if !result.Healthy {
		t.Error("health check should pass for running core")
	}
	if !result.ProcessOK {
		t.Error("ProcessOK should be true")
	}
	if !result.PortOK {
		t.Error("PortOK should be true")
	}
	if result.LatencyMs <= 0 {
		t.Error("latency should be positive")
	}

	t.Logf("Health check: healthy=%v latency=%dms", result.Healthy, result.LatencyMs)

	host.Stop(ctx, false, 3*time.Second)

	result = host.HealthCheck(ctx)
	if result.Healthy {
		t.Error("health check should fail after stop")
	}
}

func isPortInUse(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return true
	}
	ln.Close()
	return false
}
