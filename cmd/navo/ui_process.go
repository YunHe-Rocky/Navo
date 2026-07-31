package main

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"navo/internal/pipe"
)

type uiProcessManager struct {
	focus  func() bool
	start  func() (*managedProcess, error)
	events chan error

	mu      sync.Mutex
	process *managedProcess
}

func newUIProcessManager(executable string, output io.Writer) *uiProcessManager {
	return &uiProcessManager{
		focus: focusExistingWindow,
		start: func() (*managedProcess, error) {
			return startManagedProcess(
				exec.Command(executable),
				filepath.Dir(executable),
				output,
				false,
			)
		},
		events: make(chan error, 1),
	}
}

func (m *uiProcessManager) Show() error {
	if m.focus != nil && m.focus() {
		return nil
	}

	m.mu.Lock()
	if m.process != nil {
		select {
		case <-m.process.done:
			m.process = nil
		default:
			m.mu.Unlock()
			return nil
		}
	}
	process, err := m.start()
	if err != nil {
		m.mu.Unlock()
		return err
	}
	m.process = process
	m.mu.Unlock()

	log.Printf("[navo] desktop UI started pid=%d", process.cmd.Process.Pid)
	go m.observe(process)
	go focusStartedUI(process, m.focus, 5*time.Second)
	return nil
}

func focusStartedUI(process *managedProcess, focus func() bool, timeout time.Duration) {
	if process == nil || focus == nil {
		return
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if focus() {
			return
		}
		select {
		case <-process.done:
			return
		case <-timer.C:
			log.Printf("[navo] WARNING: desktop UI process started without a discoverable window")
			return
		case <-ticker.C:
		}
	}
}

func (m *uiProcessManager) observe(process *managedProcess) {
	err := process.waitError()
	m.mu.Lock()
	if m.process == process {
		m.process = nil
	}
	m.mu.Unlock()

	if err != nil {
		log.Printf("[navo] desktop UI exited: %v", err)
	} else {
		log.Printf("[navo] desktop UI closed")
	}
	select {
	case m.events <- err:
	default:
	}
}

func (m *uiProcessManager) Events() <-chan error {
	return m.events
}

func (m *uiProcessManager) Stop(timeout time.Duration) {
	m.mu.Lock()
	process := m.process
	m.mu.Unlock()
	if process == nil {
		return
	}

	select {
	case <-process.done:
		return
	default:
	}
	if process.cmd.Process != nil {
		_ = process.cmd.Process.Kill()
	}
	select {
	case <-process.done:
	case <-time.After(timeout):
		log.Printf("[navo] WARNING: desktop UI did not exit within %s", timeout)
	}
}

func requestExistingUI(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		channel, err := pipe.Dial(uiPipeName)
		if err != nil {
			lastErr = err
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if err := channel.SetDeadline(deadline); err != nil {
			channel.Close()
			return fmt.Errorf("set UI request deadline: %w", err)
		}
		request := map[string]interface{}{
			"request_id": fmt.Sprintf("show-ui-%d", time.Now().UnixNano()),
			"type":       "REQUEST",
			"method":     "ui.show",
		}
		if err := channel.Send(request); err != nil {
			channel.Close()
			return fmt.Errorf("send UI request: %w", err)
		}
		var response wireUIResponse
		if err := channel.Receive(&response); err != nil {
			channel.Close()
			return fmt.Errorf("receive UI response: %w", err)
		}
		channel.Close()
		if response.Type == "ERROR" {
			return fmt.Errorf("existing instance rejected UI request")
		}
		return nil
	}
	return fmt.Errorf("connect to existing instance: %w", lastErr)
}

type wireUIResponse struct {
	Type string `json:"type"`
}
