package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
)

const serviceIPCReplayLimit = 256

type ipcReplayEntry struct {
	fingerprint [sha256.Size]byte
	ready       chan struct{}
	response    map[string]interface{}
}

// ipcReplayCache gives each request ID at-most-once execution semantics within
// the Service process. Agent reconnects can safely retrieve the first response.
type ipcReplayCache struct {
	mu        sync.Mutex
	entries   map[string]*ipcReplayEntry
	completed []string
}

func fingerprintIPCRequest(msg map[string]interface{}) ([sha256.Size]byte, error) {
	encoded, err := json.Marshal(msg)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}

func (c *ipcReplayCache) execute(
	ctx context.Context,
	requestID string,
	fingerprint [sha256.Size]byte,
	handler func() map[string]interface{},
) map[string]interface{} {
	c.mu.Lock()
	if c.entries == nil {
		c.entries = make(map[string]*ipcReplayEntry)
	}
	if existing, ok := c.entries[requestID]; ok {
		if existing.fingerprint != fingerprint {
			c.mu.Unlock()
			return errorResponse(requestID, "REQUEST_ID_REUSE", fmt.Errorf("request_id %q was already used for a different request", requestID))
		}
		ready := existing.ready
		c.mu.Unlock()
		select {
		case <-ready:
			return cloneIPCResponse(existing.response)
		case <-ctx.Done():
			return errorResponse(requestID, "REQUEST_REPLAY_WAIT_FAILED", ctx.Err())
		}
	}

	entry := &ipcReplayEntry{
		fingerprint: fingerprint,
		ready:       make(chan struct{}),
	}
	c.entries[requestID] = entry
	c.mu.Unlock()

	result := func() (response map[string]interface{}) {
		defer func() {
			if recovered := recover(); recovered != nil {
				response = errorResponse(
					requestID,
					"INTERNAL",
					fmt.Errorf("IPC handler panic: %v", recovered),
				)
			}
		}()
		return handler()
	}()

	c.mu.Lock()
	entry.response = cloneIPCResponse(result)
	close(entry.ready)
	c.completed = append(c.completed, requestID)
	for len(c.completed) > serviceIPCReplayLimit {
		expiredID := c.completed[0]
		c.completed = c.completed[1:]
		delete(c.entries, expiredID)
	}
	c.mu.Unlock()

	return result
}

func cloneIPCResponse(response map[string]interface{}) map[string]interface{} {
	encoded, err := json.Marshal(response)
	if err != nil {
		return response
	}
	var cloned map[string]interface{}
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return response
	}
	return cloned
}
