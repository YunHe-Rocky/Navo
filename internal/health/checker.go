// Package health provides periodic health checking for the proxy core.
package health

import (
	"context"
	"log"
	"sync"
	"time"

	"navo/internal/host"
)

// Checker performs periodic health checks on a CoreHost.
type Checker struct {
	host       host.CoreHost
	interval   time.Duration
	onUnhealthy func(*host.HealthResult)

	mu       sync.Mutex
	ticker   *time.Ticker
	stopCh   chan struct{}
	running  bool
	lastResult *host.HealthResult
}

// NewChecker creates a new health Checker.
// onUnhealthy is called when a health check fails (may be nil).
func NewChecker(h host.CoreHost, interval time.Duration, onUnhealthy func(*host.HealthResult)) *Checker {
	return &Checker{
		host:       h,
		interval:   interval,
		onUnhealthy: onUnhealthy,
		stopCh:     make(chan struct{}),
	}
}

// Start begins periodic health checking.
func (c *Checker) Start(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return
	}

	c.running = true
	c.ticker = time.NewTicker(c.interval)

	go func() {
		// Run first check immediately
		c.check(ctx)

		for {
			select {
			case <-c.ticker.C:
				c.check(ctx)
			case <-c.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	log.Printf("[health] checker started, interval=%v", c.interval)
}

// Stop stops periodic health checking.
func (c *Checker) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	c.running = false
	if c.ticker != nil {
		c.ticker.Stop()
	}
	close(c.stopCh)
	log.Printf("[health] checker stopped")
}

// LastResult returns the most recent health check result.
func (c *Checker) LastResult() *host.HealthResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastResult
}

// check performs a single health check.
func (c *Checker) check(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result := c.host.HealthCheck(checkCtx)

	c.mu.Lock()
	c.lastResult = result
	c.mu.Unlock()

	if result.Healthy {
		log.Printf("[health] OK (latency=%dms)", result.LatencyMs)
	} else {
		log.Printf("[health] FAILED: %s", result.Error)
		if c.onUnhealthy != nil {
			c.onUnhealthy(result)
		}
	}
}
