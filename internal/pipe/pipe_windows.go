//go:build windows

package pipe

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procCreateNamedPipeW    = kernel32.NewProc("CreateNamedPipeW")
	procConnectNamedPipe    = kernel32.NewProc("ConnectNamedPipe")
	procWaitNamedPipeW      = kernel32.NewProc("WaitNamedPipeW")
	procCreateFileW         = kernel32.NewProc("CreateFileW")
	procCreateEventW        = kernel32.NewProc("CreateEventW")
	procGetOverlappedResult = kernel32.NewProc("GetOverlappedResult")
	procCloseHandle         = kernel32.NewProc("CloseHandle")
	procCancelIoEx          = kernel32.NewProc("CancelIoEx")
	procWaitForSingleObject = kernel32.NewProc("WaitForSingleObject")
	procLocalFree           = kernel32.NewProc("LocalFree")
	procConvertSDDL         = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
)

const (
	pipeAccessDuplex       = 0x00000003
	pipeTypeMessage        = 0x00000004
	pipeReadmodeMessage    = 0x00000002
	pipeWait               = 0x00000000
	pipeUnlimitedInstances = 255

	fileFlagOverlapped  = 0x40000000
	genericRead         = 0x80000000
	genericWrite        = 0x40000000
	openExisting        = 3
	fileAttributeNormal = 0x00000080

	errorPipeConnected = 535
	errorPipeBusy      = 231

	waitTimeout   = 0x00000102 // WAIT_TIMEOUT
	waitFailed    = 0xFFFFFFFF
	sddlRevision1 = 1
)

type securityAttributes struct {
	length             uint32
	securityDescriptor uintptr
	inheritHandle      int32
}

// errPipeTimeout is returned when a read/write deadline is exceeded.
var errPipeTimeout = os.NewSyscallError("i/o timeout", syscall.ERROR_OPERATION_ABORTED)

// ── Named Pipe Listener (multi-instance, concurrent) ──

// NamedPipeListener listens on a Windows named pipe with multiple instances
// listening concurrently. Each instance calls ConnectNamedPipe in its own
// goroutine so that concurrent client connections never see ERROR_PIPE_BUSY.
type NamedPipeListener struct {
	name   string
	path   string
	connCh chan acceptResult
	done   chan struct{}
	wg     sync.WaitGroup

	lifecycleMu sync.Mutex
	closing     bool
	closeOnce   sync.Once
}

type acceptResult struct {
	conn *pipeConn
	err  error
}

// NewListener creates a new named pipe listener with concurrent accept.
func NewListener(pipeName string) (*NamedPipeListener, error) {
	path := `\\.\pipe\` + pipeName

	l := &NamedPipeListener{
		name:   pipeName,
		path:   path,
		connCh: make(chan acceptResult, 64),
		done:   make(chan struct{}),
	}

	// Start enough listening instances for concurrent client connections.
	for i := 0; i < 32; i++ {
		l.startInstance()
	}

	return l, nil
}

func (l *NamedPipeListener) startInstance() {
	l.lifecycleMu.Lock()
	defer l.lifecycleMu.Unlock()
	if l.closing {
		return
	}
	l.wg.Add(1)
	go l.listenLoop()
}

func (l *NamedPipeListener) listenLoop() {
	defer l.wg.Done()

	for {
		select {
		case <-l.done:
			return
		default:
		}

		h, err := createNamedPipe(l.path)
		if err != nil {
			select {
			case <-l.done:
				return
			case <-time.After(500 * time.Millisecond):
			}
			continue
		}

		overlapped := &syscall.Overlapped{}
		hEvent, eventErr := createEvent()
		if eventErr != nil {
			procCloseHandle.Call(uintptr(h))
			continue
		}
		overlapped.HEvent = hEvent

		ret, _, callErr := procConnectNamedPipe.Call(
			uintptr(h),
			uintptr(unsafe.Pointer(overlapped)),
		)

		connected := false
		if ret != 0 {
			connected = true
		} else {
			errno, _ := callErr.(syscall.Errno)
			if errno == syscall.Errno(errorPipeConnected) {
				connected = true
			} else if errno == syscall.ERROR_IO_PENDING {
				// Poll with 500ms timeout so we can check l.done
				for {
					waitRet, _, _ := procWaitForSingleObject.Call(
						uintptr(hEvent), uintptr(500))
					if waitRet == waitTimeout {
						select {
						case <-l.done:
							cancelAndReap(h, overlapped, hEvent)
							procCloseHandle.Call(uintptr(hEvent))
							procCloseHandle.Call(uintptr(h))
							return
						default:
						}
						continue
					}
					// WAIT_OBJECT_0 (0) or error — stop polling
					if waitRet == 0 {
						_, connectedErr := getOverlappedResult(h, overlapped)
						connected = connectedErr == nil
					}
					break
				}
			}
		}

		procCloseHandle.Call(uintptr(hEvent))

		if !connected {
			procCloseHandle.Call(uintptr(h))
			continue
		}

		conn := newPipeConn(h, l.path)
		select {
		case l.connCh <- acceptResult{conn: conn}:
		case <-l.done:
			conn.Close()
			return
		}
	}
}

// Accept waits for and returns the next client connection.
func (l *NamedPipeListener) Accept() (*Channel, error) {
	select {
	case result, ok := <-l.connCh:
		if !ok {
			return nil, fmt.Errorf("listener closed")
		}
		if result.err != nil {
			return nil, result.err
		}
		return NewChannel(NewConn(result.conn), l.name), nil
	case <-l.done:
		return nil, fmt.Errorf("listener closed")
	}
}

// Close stops the listener and cleans up all resources.
func (l *NamedPipeListener) Close() error {
	l.closeOnce.Do(func() {
		l.lifecycleMu.Lock()
		l.closing = true
		close(l.done)
		l.lifecycleMu.Unlock()

		// Drain connCh in background so listenLoop goroutines blocked on send
		// can unblock, observe l.done, and exit.
		drainDone := make(chan struct{})
		go func() {
			for result := range l.connCh {
				if result.conn != nil {
					result.conn.Close()
				}
			}
			close(drainDone)
		}()

		l.wg.Wait()
		close(l.connCh)
		<-drainDone
	})
	return nil
}

// Addr returns the pipe path.
func (l *NamedPipeListener) Addr() string {
	return l.path
}

// PreCreateInstances launches additional listening goroutines so that
// concurrent Flutter requests never hit ERROR_PIPE_BUSY.
func (l *NamedPipeListener) PreCreateInstances(n int) {
	for i := 0; i < n; i++ {
		l.startInstance()
	}
}

// ── Pipe Connection (with deadline support) ──

// pipeConn wraps a syscall.Handle as a read/write deadline-aware connection.
type pipeConn struct {
	handle        syscall.Handle
	name          string
	readDeadline  time.Time
	writeDeadline time.Time
	mu            sync.Mutex
	pending       map[*pendingIO]struct{}
	closed        bool
	closeOnce     sync.Once
	closeDone     chan struct{}
	closeErr      error
}

type pendingIO struct {
	overlapped *syscall.Overlapped
	event      syscall.Handle
	done       chan struct{}
}

func newPipeConn(h syscall.Handle, name string) *pipeConn {
	return &pipeConn{
		handle: h, name: name,
		pending: make(map[*pendingIO]struct{}), closeDone: make(chan struct{}),
	}
}

func (c *pipeConn) deadlineTimeout(deadline time.Time) uint32 {
	if deadline.IsZero() {
		return syscall.INFINITE
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0
	}
	ms := uint32(remaining.Milliseconds())
	if ms == 0 {
		ms = 1 // at least 1 ms
	}
	return ms
}

func (c *pipeConn) Read(b []byte) (int, error) {
	c.mu.Lock()
	timeout := c.deadlineTimeout(c.readDeadline)
	c.mu.Unlock()

	if timeout == 0 {
		return 0, errPipeTimeout
	}

	op, err := c.beginIO()
	if err != nil {
		return 0, err
	}
	defer c.endIO(op)

	var done uint32
	readErr := syscall.ReadFile(c.handle, b, &done, op.overlapped)
	if readErr != nil && readErr != syscall.ERROR_IO_PENDING {
		return int(done), readErr
	}
	if readErr == syscall.ERROR_IO_PENDING {
		transferred, getErr := awaitOverlapped(c.handle, op.overlapped, op.event, timeout)
		if getErr != nil {
			return int(transferred), getErr
		}
		done = transferred
	}
	return int(done), nil
}

func (c *pipeConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	timeout := c.deadlineTimeout(c.writeDeadline)
	c.mu.Unlock()

	if timeout == 0 {
		return 0, errPipeTimeout
	}

	op, err := c.beginIO()
	if err != nil {
		return 0, err
	}
	defer c.endIO(op)

	var done uint32
	writeErr := syscall.WriteFile(c.handle, b, &done, op.overlapped)
	if writeErr != nil && writeErr != syscall.ERROR_IO_PENDING {
		return int(done), writeErr
	}
	if writeErr == syscall.ERROR_IO_PENDING {
		transferred, getErr := awaitOverlapped(c.handle, op.overlapped, op.event, timeout)
		if getErr != nil {
			return int(transferred), getErr
		}
		done = transferred
	}
	return int(done), nil
}

func (c *pipeConn) Close() error {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		pending := make([]*pendingIO, 0, len(c.pending))
		for op := range c.pending {
			pending = append(pending, op)
		}
		c.mu.Unlock()

		for _, op := range pending {
			procCancelIoEx.Call(
				uintptr(c.handle), uintptr(unsafe.Pointer(op.overlapped)),
			)
		}
		for _, op := range pending {
			<-op.done
		}
		c.closeErr = syscall.CloseHandle(c.handle)
		close(c.closeDone)
	})
	<-c.closeDone
	return c.closeErr
}

func (c *pipeConn) beginIO() (*pendingIO, error) {
	event, err := createEvent()
	if err != nil {
		return nil, err
	}
	op := &pendingIO{
		overlapped: &syscall.Overlapped{HEvent: event},
		event:      event,
		done:       make(chan struct{}),
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		procCloseHandle.Call(uintptr(event))
		return nil, windows.ERROR_INVALID_HANDLE
	}
	c.pending[op] = struct{}{}
	return op, nil
}

func (c *pipeConn) endIO(op *pendingIO) {
	c.mu.Lock()
	delete(c.pending, op)
	c.mu.Unlock()
	procCloseHandle.Call(uintptr(op.event))
	close(op.done)
}

func (c *pipeConn) LocalAddr() pipeAddr  { return pipeAddr(c.name) }
func (c *pipeConn) RemoteAddr() pipeAddr { return pipeAddr(c.name) }

func (c *pipeConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	c.readDeadline = t
	c.writeDeadline = t
	c.mu.Unlock()
	return nil
}

func (c *pipeConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	c.readDeadline = t
	c.mu.Unlock()
	return nil
}

func (c *pipeConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	c.writeDeadline = t
	c.mu.Unlock()
	return nil
}

type pipeAddr string

func (a pipeAddr) Network() string { return "pipe" }
func (a pipeAddr) String() string  { return string(a) }

// ── Client ──

// Dial connects to a named pipe server.
func Dial(pipeName string) (*Channel, error) {
	path := `\\.\pipe\` + pipeName
	return DialPath(path)
}

// DialPath connects to a full named pipe path.
func DialPath(path string) (*Channel, error) {
	namePtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	var h syscall.Handle
	for i := 0; i < 20; i++ {
		ret, _, callErr := procCreateFileW.Call(
			uintptr(unsafe.Pointer(namePtr)),
			uintptr(genericRead|genericWrite),
			uintptr(0),
			uintptr(0),
			uintptr(openExisting),
			uintptr(fileAttributeNormal|fileFlagOverlapped),
			uintptr(0),
		)
		if ret != uintptr(syscall.InvalidHandle) {
			h = syscall.Handle(ret)
			break
		}
		var errno syscall.Errno
		if callErr != nil {
			if e, ok := callErr.(syscall.Errno); ok {
				errno = e
			}
		}
		if errno == syscall.Errno(0) && ret == uintptr(syscall.InvalidHandle) {
			if le, ok := syscall.GetLastError().(syscall.Errno); ok {
				errno = le
			}
		}
		if errno != syscall.Errno(errorPipeBusy) && errno != syscall.ERROR_FILE_NOT_FOUND {
			return nil, fmt.Errorf("CreateFile(%s): errno=%d", path, errno)
		}
		procWaitNamedPipeW.Call(uintptr(unsafe.Pointer(namePtr)), uintptr(200))
	}

	if h == 0 {
		return nil, fmt.Errorf("pipe not available: %s", path)
	}

	conn := newPipeConn(h, path)
	return NewChannel(NewConn(conn), path), nil
}

// ── Helpers ──

func createNamedPipe(path string) (syscall.Handle, error) {
	namePtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	security, descriptor, err := pipeSecurityAttributes()
	if err != nil {
		return 0, err
	}
	defer procLocalFree.Call(descriptor)

	h, _, e := procCreateNamedPipeW.Call(
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(pipeAccessDuplex|fileFlagOverlapped),
		uintptr(pipeTypeMessage|pipeReadmodeMessage|pipeWait),
		uintptr(pipeUnlimitedInstances),
		uintptr(65536),
		uintptr(65536),
		uintptr(unsafe.Pointer(security)),
		uintptr(0),
	)
	if h == uintptr(syscall.InvalidHandle) {
		return 0, fmt.Errorf("CreateNamedPipe failed: %v", e)
	}

	return syscall.Handle(h), nil
}

func pipeSecurityAttributes() (*securityAttributes, uintptr, error) {
	sddlString, err := pipeSecuritySDDL()
	if err != nil {
		return nil, 0, err
	}
	sddl, err := syscall.UTF16PtrFromString(sddlString)
	if err != nil {
		return nil, 0, err
	}
	var descriptor uintptr
	result, _, callErr := procConvertSDDL.Call(
		uintptr(unsafe.Pointer(sddl)),
		uintptr(sddlRevision1),
		uintptr(unsafe.Pointer(&descriptor)),
		uintptr(0),
	)
	if result == 0 {
		return nil, 0, fmt.Errorf("create named pipe security descriptor: %w", callErr)
	}
	attributes := &securityAttributes{
		length:             uint32(unsafe.Sizeof(securityAttributes{})),
		securityDescriptor: descriptor,
	}
	return attributes, descriptor, nil
}

func pipeSecuritySDDL() (string, error) {
	var token windows.Token
	if err := windows.OpenProcessToken(
		windows.CurrentProcess(),
		windows.TOKEN_QUERY,
		&token,
	); err != nil {
		return "", fmt.Errorf("open process token for named pipe ACL: %w", err)
	}
	defer token.Close()

	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("read process user for named pipe ACL: %w", err)
	}
	userSID := tokenUser.User.Sid.String()
	if userSID == "" {
		return "", fmt.Errorf("read process user for named pipe ACL: empty SID")
	}

	// OW may resolve to Administrators for an elevated process. An explicit
	// token-user SID permits only the same signed-in user across integrity levels.
	return fmt.Sprintf(
		"D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;%s)",
		userSID,
	), nil
}

func createEvent() (syscall.Handle, error) {
	h, _, e := procCreateEventW.Call(
		uintptr(0), uintptr(0), uintptr(0), uintptr(0),
	)
	if h == 0 {
		return 0, fmt.Errorf("CreateEvent failed: %v", e)
	}
	return syscall.Handle(h), nil
}

func awaitOverlapped(
	h syscall.Handle,
	overlapped *syscall.Overlapped,
	event syscall.Handle,
	timeout uint32,
) (uint32, error) {
	waitResult, _, waitErr := procWaitForSingleObject.Call(
		uintptr(event), uintptr(timeout),
	)
	switch waitResult {
	case 0:
		return getOverlappedResult(h, overlapped)
	case waitTimeout:
		cancelAndReap(h, overlapped, event)
		return 0, errPipeTimeout
	case waitFailed:
		cancelAndReap(h, overlapped, event)
		return 0, fmt.Errorf("WaitForSingleObject failed: %w", waitErr)
	default:
		cancelAndReap(h, overlapped, event)
		return 0, fmt.Errorf("unexpected overlapped wait result: %d", waitResult)
	}
}

// cancelAndReap keeps the OVERLAPPED and event alive until the kernel has
// completed cancellation. Callers may release those resources only afterward.
func cancelAndReap(
	h syscall.Handle,
	overlapped *syscall.Overlapped,
	event syscall.Handle,
) {
	procCancelIoEx.Call(uintptr(h), uintptr(unsafe.Pointer(overlapped)))
	procWaitForSingleObject.Call(uintptr(event), uintptr(syscall.INFINITE))
	_, _ = getOverlappedResult(h, overlapped)
}

func getOverlappedResult(h syscall.Handle, overlapped *syscall.Overlapped) (uint32, error) {
	var transferred uint32
	ret, _, e := procGetOverlappedResult.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(overlapped)),
		uintptr(unsafe.Pointer(&transferred)),
		uintptr(0), // bWait = false
	)
	if ret == 0 {
		return 0, fmt.Errorf("GetOverlappedResult failed: %v", e)
	}
	return transferred, nil
}
