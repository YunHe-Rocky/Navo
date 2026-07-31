//go:build windows

package winprocess

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
)

const hiddenProcessHelper = "NAVO_HIDDEN_PROCESS_HELPER"

func TestConfigureHiddenSetsRequiredWindowsFlags(t *testing.T) {
	command := exec.Command("cmd.exe", "/c", "exit", "0")
	ConfigureHidden(command)
	if command.SysProcAttr == nil {
		t.Fatal("SysProcAttr was not initialized")
	}
	if !command.SysProcAttr.HideWindow {
		t.Fatal("HideWindow was not enabled")
	}
	if command.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatal("CREATE_NO_WINDOW was not enabled")
	}
}

func TestConfiguredProcessHasNoConsoleWindow(t *testing.T) {
	if os.Getenv(hiddenProcessHelper) == "1" {
		window, _, _ := syscall.NewLazyDLL("kernel32.dll").
			NewProc("GetConsoleWindow").
			Call()
		fmt.Println(window)
		os.Exit(0)
	}
	command := exec.Command(os.Args[0], "-test.run=TestConfiguredProcessHasNoConsoleWindow")
	command.Env = append(os.Environ(), hiddenProcessHelper+"=1")
	ConfigureHidden(command)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("hidden process failed: %v: %s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != "0" {
		t.Fatalf("hidden process console handle = %q, want 0", got)
	}
}
