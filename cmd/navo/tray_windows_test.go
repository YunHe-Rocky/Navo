//go:build windows

package main

import "testing"

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
