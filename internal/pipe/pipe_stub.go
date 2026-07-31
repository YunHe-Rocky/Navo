//go:build !windows

package pipe

import (
	"fmt"
	"net"
	"os"
)

// NewListener creates a named pipe listener (Unix domain socket fallback for testing).
func NewListener(pipeName string) (Listener, error) {
	// On non-Windows, use a Unix domain socket in the temp directory
	socketPath := os.TempDir() + "/navo-" + pipeName + ".sock"
	os.Remove(socketPath) // clean up stale socket

	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("unix listen: %w", err)
	}

	return &unixListener{listener: l, path: socketPath, name: pipeName}, nil
}

type unixListener struct {
	listener net.Listener
	path     string
	name     string
}

func (l *unixListener) Accept() (*Channel, error) {
	conn, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}
	return NewChannel(NewConn(conn), l.name), nil
}

func (l *unixListener) Close() error {
	os.Remove(l.path)
	return l.listener.Close()
}

func (l *unixListener) Addr() string {
	return l.path
}

// Dial connects to a named pipe (Unix domain socket fallback).
func Dial(pipeName string) (*Channel, error) {
	socketPath := os.TempDir() + "/navo-" + pipeName + ".sock"
	return DialPath(socketPath)
}

// DialPath connects to a specific path (Unix domain socket).
func DialPath(path string) (*Channel, error) {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, fmt.Errorf("dial unix %s: %w", path, err)
	}
	return NewChannel(NewConn(conn), path), nil
}
