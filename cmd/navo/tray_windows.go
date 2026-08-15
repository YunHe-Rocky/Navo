//go:build windows

package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

//go:embed winres/navo.ico
var embeddedIcon []byte

const (
	wmApp      = 0x8000
	wmTrayIcon = wmApp + 1
	wmTrayShow = wmApp + 2

	nimAdd        = 0x00000000
	nimModify     = 0x00000001
	nimDelete     = 0x00000002
	nimSetVersion = 0x00000004
	nifMessage    = 0x00000001
	nifIcon       = 0x00000002
	nifTip        = 0x00000004
	nifInfo       = 0x00000010
	notifyIconV4  = 4
	niifInfo      = 0x00000001

	tpmLeftAlign   = 0x0000
	tpmBottomAlign = 0x0020
	tpmReturnCmd   = 0x0100

	mfString    = 0x00000000
	mfGrayed    = 0x00000001
	mfDisabled  = 0x00000002
	mfChecked   = 0x00000008
	mfPopup     = 0x00000010
	mfSeparator = 0x00000800
	mfDefault   = 0x00001000

	wmDestroy   = 0x0002
	wmClose     = 0x0010
	wmLButtonUp = 0x0202
	wmRButtonUp = 0x0205

	swRestore = 9

	imageIcon      = 1
	lrLoadFromFile = 0x00000010
	lrDefaultColor = 0x00000000

	trayDynamicCommandBase = 1000

	mbOK            = 0x00000000
	mbIconError     = 0x00000010
	mbIconInfo      = 0x00000040
	mbSetForeground = 0x00010000
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procRegisterWindowMsgW  = user32.NewProc("RegisterWindowMessageW")
	procFindWindowW         = user32.NewProc("FindWindowW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procLoadImageW          = user32.NewProc("LoadImageW")
	procDestroyIcon         = user32.NewProc("DestroyIcon")
	procMessageBoxW         = user32.NewProc("MessageBoxW")

	procGetModuleHandleW       = kernel32.NewProc("GetModuleHandleW")
	procShellNotifyIconW       = shell32.NewProc("Shell_NotifyIconW")
	procShellNotifyIconGetRect = shell32.NewProc("Shell_NotifyIconGetRect")
)

type notifyIconData struct {
	cbSize           uint32
	hwnd             syscall.Handle
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            syscall.Handle
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     syscall.Handle
}

type point struct {
	x int32
	y int32
}

type rect struct {
	left   int32
	top    int32
	right  int32
	bottom int32
}

type notifyIconIdentifier struct {
	cbSize   uint32
	hwnd     syscall.Handle
	uID      uint32
	guidItem [16]byte
}

type windowClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}

type windowMessage struct {
	hwnd    syscall.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type trayController struct {
	hwnd      syscall.Handle
	done      <-chan struct{}
	refresh   <-chan error
	refreshMu sync.Mutex
	once      sync.Once
}

func (t *trayController) Close(timeout time.Duration) {
	if t == nil {
		return
	}
	t.once.Do(func() {
		procPostMessageW.Call(uintptr(t.hwnd), wmClose, 0, 0)
	})
	select {
	case <-t.done:
	case <-time.After(timeout):
		log.Printf("[navo] tray shutdown timed out")
	}
}

// EnsureVisible synchronously refreshes the shell icon before the UI hides.
// If Explorer has recreated its taskbar, the tray window re-adds the icon.
func (t *trayController) EnsureVisible() error {
	if t == nil || t.hwnd == 0 {
		return fmt.Errorf("native tray is unavailable")
	}
	t.refreshMu.Lock()
	defer t.refreshMu.Unlock()
	ok, _, callErr := procPostMessageW.Call(uintptr(t.hwnd), wmTrayShow, 0, 0)
	if ok == 0 {
		return fmt.Errorf("queue native tray refresh: %w", callErr)
	}
	select {
	case err := <-t.refresh:
		return err
	case <-t.done:
		return fmt.Errorf("native tray stopped during refresh")
	case <-time.After(2 * time.Second):
		return fmt.Errorf("native tray refresh timed out")
	}
}

type trayReady struct {
	hwnd syscall.Handle
	err  error
}

// startTray creates the application's sole tray icon and owns its message loop.
// It first tries to write the embedded icon to a temp file; falls back to searching
// common paths if the embedded icon is not available.
func startTray(
	iconPath string,
	onShow func(),
	onExit func(),
	backend trayBackend,
) (*trayController, error) {
	ready := make(chan trayReady, 1)
	done := make(chan struct{})
	refresh := make(chan error, 1)

	go func() {
		defer close(done)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		actualPath := iconPath

		// Try embedded icon first: write to temp dir next to executable
		if len(embeddedIcon) > 0 {
			exe, _ := os.Executable()
			tempDir := filepath.Join(filepath.Dir(exe), "data")
			_ = os.MkdirAll(tempDir, 0700)
			tempIcon := filepath.Join(tempDir, "navo.ico")
			if _, err := os.Stat(tempIcon); os.IsNotExist(err) {
				_ = os.WriteFile(tempIcon, embeddedIcon, 0644)
			}
			if _, err := os.Stat(tempIcon); err == nil {
				actualPath = tempIcon
			}
		}

		// Fallback: search relative to exe directory
		if actualPath == iconPath {
			if _, err := os.Stat(actualPath); os.IsNotExist(err) {
				exe, _ := os.Executable()
				candidates := []string{
					filepath.Join(filepath.Dir(exe), "navo.ico"),
					filepath.Join(filepath.Dir(exe), "app_ui", "tray_icon.ico"),
				}
				for _, c := range candidates {
					if _, err2 := os.Stat(c); err2 == nil {
						actualPath = c
						break
					}
				}
			}
		}

		iconPtr, err := syscall.UTF16PtrFromString(actualPath)
		if err != nil {
			ready <- trayReady{err: fmt.Errorf("encode tray icon path: %w", err)}
			return
		}
		hIcon, _, _ := procLoadImageW.Call(
			0,
			uintptr(unsafe.Pointer(iconPtr)),
			imageIcon,
			0,
			0,
			lrLoadFromFile|lrDefaultColor,
		)
		if hIcon == 0 {
			ready <- trayReady{err: fmt.Errorf("load tray icon from %s: %w", actualPath, syscall.GetLastError())}
			return
		}
		defer procDestroyIcon.Call(hIcon)

		hInstance, _, _ := procGetModuleHandleW.Call(0)
		if hInstance == 0 {
			ready <- trayReady{err: fmt.Errorf("get module handle: %w", syscall.GetLastError())}
			return
		}

		className, _ := syscall.UTF16PtrFromString("NavoTrayClass")
		wc := windowClassEx{
			cbSize:        uint32(unsafe.Sizeof(windowClassEx{})),
			lpfnWndProc:   syscall.NewCallback(trayWndProc),
			hInstance:     syscall.Handle(hInstance),
			lpszClassName: className,
		}
		if atom, _, callErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
			ready <- trayReady{err: fmt.Errorf("register tray window class: %w", callErr)}
			return
		}

		hwnd, _, callErr := procCreateWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(className)),
			0,
			0,
			0,
			0,
			0,
			0, // hidden top-level window; no WS_VISIBLE style
			0,
			0,
			0,
			0,
			hInstance,
			0,
		)
		if hwnd == 0 {
			ready <- trayReady{err: fmt.Errorf("create tray window: %w", callErr)}
			return
		}

		trayOnShow = onShow
		trayOnExit = onExit
		trayBackendInstance = backend
		trayRefreshResults = refresh
		defer func() { trayRefreshResults = nil }()

		trayIconHandle = syscall.Handle(hIcon)
		defer func() { trayIconHandle = 0 }()
		taskbarCreatedName, _ := syscall.UTF16PtrFromString("TaskbarCreated")
		registeredMessage, _, _ := procRegisterWindowMsgW.Call(uintptr(unsafe.Pointer(taskbarCreatedName)))
		trayTaskbarCreatedMessage = uint32(registeredMessage)
		defer func() { trayTaskbarCreatedMessage = 0 }()

		nid := newNotifyIconData(syscall.Handle(hwnd), syscall.Handle(hIcon), false)

		if added, _, callErr := procShellNotifyIconW.Call(
			nimAdd,
			uintptr(unsafe.Pointer(&nid)),
		); added == 0 {
			procDestroyWindow.Call(hwnd)
			ready <- trayReady{err: fmt.Errorf("add tray icon: %w", callErr)}
			return
		}
		defer procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))

		nid.uVersion = notifyIconV4
		procShellNotifyIconW.Call(nimSetVersion, uintptr(unsafe.Pointer(&nid)))
		ready <- trayReady{hwnd: syscall.Handle(hwnd)}

		var msg windowMessage
		for {
			result, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if result == 0 || result == ^uintptr(0) {
				return
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()

	result := <-ready
	if result.err != nil {
		<-done
		return nil, result.err
	}
	return &trayController{hwnd: result.hwnd, done: done, refresh: refresh}, nil
}

var (
	trayOnShow                func()
	trayOnExit                func()
	trayBackendInstance       trayBackend
	trayIconHandle            syscall.Handle
	trayTaskbarCreatedMessage uint32
	trayRefreshResults        chan<- error
)

func trayWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	if trayTaskbarCreatedMessage != 0 && msg == trayTaskbarCreatedMessage {
		if err := restoreTrayIcon(hwnd, false); err != nil {
			log.Printf("[navo] restore tray icon after Explorer restart: %v", err)
		}
		return 0
	}
	switch msg {
	case wmTrayIcon:
		// NOTIFYICON_VERSION_4 packs the event code into the low word.
		switch trayEventCode(lParam) {
		case wmRButtonUp:
			dispatchTrayAction(showTrayMenu(hwnd))
		case wmLButtonUp:
			dispatchTrayAction(&trayAction{Kind: trayActionOpen})
		}
		return 0
	case wmTrayShow:
		err := restoreTrayIcon(hwnd, true)
		if err != nil {
			log.Printf("[navo] refresh tray icon before minimize: %v", err)
		}
		if trayRefreshResults != nil {
			select {
			case trayRefreshResults <- err:
			default:
			}
		}
		return 0
	case wmClose:
		procDestroyWindow.Call(uintptr(hwnd))
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	result, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return result
}

func newNotifyIconData(hwnd, hIcon syscall.Handle, minimized bool) notifyIconData {
	nid := notifyIconData{
		cbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
		hwnd:             hwnd,
		uID:              1,
		uFlags:           nifMessage | nifIcon | nifTip,
		uCallbackMessage: wmTrayIcon,
		hIcon:            hIcon,
	}
	copy(nid.szTip[:], utf16Text("Navo"))
	if minimized {
		nid.uFlags |= nifInfo
		copy(nid.szInfoTitle[:], utf16Text("Navo 已最小化"))
		copy(nid.szInfo[:], utf16Text("程序仍在后台运行，点击托盘图标可重新打开。"))
		nid.dwInfoFlags = niifInfo
	}
	return nid
}

func restoreTrayIcon(hwnd syscall.Handle, minimized bool) error {
	if trayIconHandle == 0 {
		return fmt.Errorf("tray icon handle is unavailable")
	}
	nid := newNotifyIconData(hwnd, trayIconHandle, false)
	if !trayIconRegistered(hwnd) {
		added, _, callErr := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
		if added == 0 {
			return fmt.Errorf("add tray icon: %w", callErr)
		}
		nid.uVersion = notifyIconV4
		if versioned, _, callErr := procShellNotifyIconW.Call(nimSetVersion, uintptr(unsafe.Pointer(&nid))); versioned == 0 {
			return fmt.Errorf("set tray icon version: %w", callErr)
		}
	}
	if minimized {
		notification := newNotifyIconData(hwnd, trayIconHandle, true)
		notification.uFlags = nifInfo
		if shown, _, callErr := procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&notification))); shown == 0 {
			// The icon is already confirmed visible. Notifications may be disabled
			// by Windows policy, so treat this as non-fatal feedback degradation.
			log.Printf("[navo] tray minimize notification unavailable: %v", callErr)
		}
	}
	return nil
}

func trayIconRegistered(hwnd syscall.Handle) bool {
	identifier := notifyIconIdentifier{
		cbSize: uint32(unsafe.Sizeof(notifyIconIdentifier{})),
		hwnd:   hwnd,
		uID:    1,
	}
	var bounds rect
	hresult, _, _ := procShellNotifyIconGetRect.Call(
		uintptr(unsafe.Pointer(&identifier)),
		uintptr(unsafe.Pointer(&bounds)),
	)
	return int32(hresult) >= 0
}

func utf16Text(value string) []uint16 {
	encoded, err := syscall.UTF16FromString(value)
	if err != nil {
		return []uint16{0}
	}
	return encoded
}

func trayEventCode(lParam uintptr) uint32 {
	return uint32(lParam & 0xffff)
}

func dispatchTrayAction(action *trayAction) {
	if action == nil {
		return
	}
	switch action.Kind {
	case trayActionOpen, trayActionSettings:
		if trayOnShow != nil {
			trayOnShow()
		}
	case trayActionExit:
		if trayOnExit != nil {
			trayOnExit()
		}
	default:
		backend := trayBackendInstance
		if backend == nil {
			showTrayResult("Navo", "托盘控制尚未就绪", true)
			return
		}
		go func(selected trayAction) {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			message, err := backend.Execute(ctx, selected)
			if err != nil {
				showTrayResult("Navo 操作失败", err.Error(), true)
				return
			}
			showTrayResult("Navo", message, false)
		}(*action)
	}
}

func showTrayMenu(hwnd syscall.Handle) *trayAction {
	var (
		snapshot traySnapshot
		err      error
	)
	if trayBackendInstance == nil {
		err = fmt.Errorf("托盘控制尚未就绪")
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		snapshot, err = trayBackendInstance.Snapshot(ctx)
		cancel()
	}

	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return nil
	}
	defer procDestroyMenu.Call(menu)

	commands := make(map[uintptr]trayAction)
	nextCommand := uintptr(trayDynamicCommandBase)
	appendTrayMenuItems(
		menu,
		buildTrayMenu(snapshot, err),
		commands,
		&nextCommand,
	)

	procSetForegroundWindow.Call(uintptr(hwnd))
	var cursor point
	if ok, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor))); ok == 0 {
		return nil
	}
	command, _, _ := procTrackPopupMenu.Call(
		menu,
		tpmLeftAlign|tpmBottomAlign|tpmReturnCmd,
		uintptr(cursor.x),
		uintptr(cursor.y),
		0,
		uintptr(hwnd),
		0,
	)
	action, ok := commands[command]
	if !ok {
		return nil
	}
	return &action
}

func appendTrayMenuItems(
	menu uintptr,
	items []trayMenuItem,
	commands map[uintptr]trayAction,
	nextCommand *uintptr,
) {
	for _, item := range items {
		if item.Separator {
			procAppendMenuW.Call(menu, mfSeparator, 0, 0)
			continue
		}

		flags := uintptr(mfString)
		if item.Disabled {
			flags |= mfDisabled | mfGrayed
		}
		if item.Checked {
			flags |= mfChecked
		}
		if item.Default {
			flags |= mfDefault
		}

		label, err := syscall.UTF16PtrFromString(item.Label)
		if err != nil {
			continue
		}
		if len(item.Children) > 0 {
			submenu, _, _ := procCreatePopupMenu.Call()
			if submenu == 0 {
				continue
			}
			appendTrayMenuItems(submenu, item.Children, commands, nextCommand)
			procAppendMenuW.Call(
				menu,
				flags|mfPopup,
				submenu,
				uintptr(unsafe.Pointer(label)),
			)
			continue
		}

		command := *nextCommand
		*nextCommand = *nextCommand + 1
		if item.Action != nil {
			commands[command] = *item.Action
		}
		procAppendMenuW.Call(
			menu,
			flags,
			command,
			uintptr(unsafe.Pointer(label)),
		)
	}
}

func showTrayResult(title, message string, failed bool) {
	titlePtr, titleErr := syscall.UTF16PtrFromString(title)
	messagePtr, messageErr := syscall.UTF16PtrFromString(
		trimMenuText(message, 4000),
	)
	if titleErr != nil || messageErr != nil {
		return
	}
	flags := uintptr(mbOK | mbIconInfo | mbSetForeground)
	if failed {
		flags = mbOK | mbIconError | mbSetForeground
	}
	procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(messagePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		flags,
	)
}

func showUIWindow() {
	className, _ := syscall.UTF16PtrFromString("NavoAppWindow")
	title, _ := syscall.UTF16PtrFromString("Navo")
	hwnd, _, _ := procFindWindowW.Call(
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
	)
	if hwnd == 0 {
		log.Printf("[navo] desktop window not found")
		return
	}
	procShowWindow.Call(hwnd, swRestore)
	procSetForegroundWindow.Call(hwnd)
}
