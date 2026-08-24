//go:build windows

package startup

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConfigureCreatesAndDeletesOwnedLoginTask(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "Navo", "navo.exe")
	statePath := filepath.Join(t.TempDir(), "startup.json")
	var calls [][]string
	instance := &manager{statePath: statePath, executablePath: executable}
	instance.run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return []byte("ok"), nil
	}

	status, err := instance.Configure(context.Background(), true, ModeSystemProxy)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || !status.Registered || status.Mode != ModeSystemProxy {
		t.Fatalf("enabled status = %#v", status)
	}
	quotedExecutable, err := quoteTaskExecutable(executable)
	if err != nil {
		t.Fatal(err)
	}
	wantCreate := []string{
		"schtasks.exe", "/Create", "/TN", taskNameForPath(executable), "/TR",
		quotedExecutable + " --silent --startup", "/SC", "ONLOGON",
		"/DELAY", "0000:15", "/RL", "HIGHEST", "/F",
	}
	if !reflect.DeepEqual(calls[0], wantCreate) {
		t.Fatalf("create call = %#v, want %#v", calls[0], wantCreate)
	}

	status, err = instance.Configure(context.Background(), false, ModeSystemProxy)
	if err != nil {
		t.Fatal(err)
	}
	if status.Enabled || status.Registered {
		t.Fatalf("disabled status = %#v", status)
	}
	wantDelete := []string{"schtasks.exe", "/Delete", "/TN", taskNameForPath(executable), "/F"}
	if !reflect.DeepEqual(calls[1], wantDelete) {
		t.Fatalf("delete call = %#v, want %#v", calls[1], wantDelete)
	}
}

func TestStatusSurfacesMissingRegisteredTask(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "navo.exe")
	instance := &manager{statePath: filepath.Join(t.TempDir(), "startup.json"), executablePath: executable}
	instance.run = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("task missing"), errors.New("exit status 1")
	}
	if _, err := instance.Configure(context.Background(), true, ModeTUN); err == nil {
		t.Fatal("create failure was accepted")
	}
}
