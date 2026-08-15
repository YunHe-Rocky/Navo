package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIPCReplayCacheExecutesConcurrentDuplicateOnce(t *testing.T) {
	var cache ipcReplayCache
	fingerprint, err := fingerprintIPCRequest(map[string]interface{}{
		"request_id": "duplicate",
		"method":     "runtime.set",
	})
	if err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	handler := func() map[string]interface{} {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return response("duplicate", map[string]interface{}{"committed": true})
	}

	results := make(chan map[string]interface{}, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- cache.execute(context.Background(), "duplicate", fingerprint, handler)
		}()
	}
	<-started
	close(release)
	wg.Wait()
	close(results)

	if calls.Load() != 1 {
		t.Fatalf("duplicate request executed %d times", calls.Load())
	}
	for result := range results {
		if result["type"] != "RESPONSE" {
			t.Fatalf("unexpected replay response: %#v", result)
		}
	}
}

func TestIPCReplayCacheCompletesEntryWhenHandlerPanics(t *testing.T) {
	var cache ipcReplayCache
	fingerprint, err := fingerprintIPCRequest(map[string]interface{}{
		"request_id": "panic-request",
		"method":     "runtime.set",
	})
	if err != nil {
		t.Fatal(err)
	}

	first := cache.execute(context.Background(), "panic-request", fingerprint, func() map[string]interface{} {
		panic("simulated handler panic")
	})
	assertInternalReplayError(t, first)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	replayed := cache.execute(ctx, "panic-request", fingerprint, func() map[string]interface{} {
		t.Fatal("poisoned replay entry executed twice")
		return nil
	})
	assertInternalReplayError(t, replayed)
}

func assertInternalReplayError(t *testing.T, result map[string]interface{}) {
	t.Helper()
	payload, _ := result["payload"].(map[string]interface{})
	if result["type"] != "ERROR" || payload["code"] != "INTERNAL" {
		t.Fatalf("panic response = %#v", result)
	}
}
