//go:build windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestTrayEventCodeUsesLowWordForNotifyIconV4(t *testing.T) {
	const iconID = uintptr(7)
	lParam := iconID<<16 | wmRButtonUp
	if got := trayEventCode(lParam); got != wmRButtonUp {
		t.Fatalf("trayEventCode() = %#x, want %#x", got, wmRButtonUp)
	}
}

func TestDispatchTrayAction(t *testing.T) {
	oldShow, oldExit := trayOnShow, trayOnExit
	t.Cleanup(func() {
		trayOnShow = oldShow
		trayOnExit = oldExit
	})

	var showCalls, exitCalls int
	trayOnShow = func() { showCalls++ }
	trayOnExit = func() { exitCalls++ }

	dispatchTrayAction(&trayAction{Kind: trayActionOpen})
	dispatchTrayAction(&trayAction{Kind: trayActionExit})
	dispatchTrayAction(nil)

	if showCalls != 1 || exitCalls != 1 {
		t.Fatalf("callbacks = show:%d exit:%d, want show:1 exit:1", showCalls, exitCalls)
	}
}

func TestMinimizedNotifyIconDataIncludesVisibleFeedback(t *testing.T) {
	nid := newNotifyIconData(1, 2, true)
	if nid.uFlags&nifInfo == 0 || nid.dwInfoFlags != niifInfo {
		t.Fatalf("notification flags = %#x, info flags = %#x", nid.uFlags, nid.dwInfoFlags)
	}
	if title := syscall.UTF16ToString(nid.szInfoTitle[:]); title != "Navo 已最小化" {
		t.Fatalf("notification title = %q", title)
	}
	if message := syscall.UTF16ToString(nid.szInfo[:]); message == "" {
		t.Fatal("notification message is empty")
	}
}

func TestTrayEnsureVisibleIntegration(t *testing.T) {
	if os.Getenv("NAVO_RUN_TRAY_INTEGRATION") != "1" {
		t.Skip("set NAVO_RUN_TRAY_INTEGRATION=1 to exercise Windows Explorer tray integration")
	}
	iconPath, err := filepath.Abs(filepath.Join("..", "..", "winres", "navo.ico"))
	if err != nil {
		t.Fatal(err)
	}
	tray, err := startTray(iconPath, func() {}, func() {}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tray.Close(2 * time.Second) })
	// Explorer publishes the new NOTIFYICON identifier asynchronously.
	time.Sleep(500 * time.Millisecond)
	if err := tray.EnsureVisible(); err != nil {
		t.Fatal(err)
	}
}
