//go:build windows

package pipe

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestNamedPipeListenerCloseIsConcurrentAndIdempotent(t *testing.T) {
	name := fmt.Sprintf("Navo.CloseTest.%d.%d", os.Getpid(), time.Now().UnixNano())
	listener, err := NewListener(name)
	if err != nil {
		t.Fatal(err)
	}

	var callers sync.WaitGroup
	for i := 0; i < 16; i++ {
		callers.Add(1)
		go func() {
			defer callers.Done()
			if err := listener.Close(); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		}()
	}
	callers.Wait()

	if _, err := listener.Accept(); err == nil {
		t.Fatal("Accept() succeeded after listener close")
	}
	listener.PreCreateInstances(1)
}

func TestPipeSecuritySDDLGrantsCurrentTokenUser(t *testing.T) {
	sddl, err := pipeSecuritySDDL()
	if err != nil {
		t.Fatal(err)
	}

	var token windows.Token
	if err := windows.OpenProcessToken(
		windows.CurrentProcess(),
		windows.TOKEN_QUERY,
		&token,
	); err != nil {
		t.Fatal(err)
	}
	defer token.Close()

	tokenUser, err := token.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	userSID := tokenUser.User.Sid.String()
	if userSID == "" || !strings.Contains(sddl, "(A;;GA;;;"+userSID+")") {
		t.Fatalf("current token user is absent from pipe DACL: %q", sddl)
	}
	if strings.Contains(sddl, ";;;WD)") || strings.Contains(sddl, ";;;AU)") {
		t.Fatalf("pipe DACL grants broad local access: %q", sddl)
	}
}

func TestNamedPipeReadTimeoutCancelsAndConnectionRecovers(t *testing.T) {
	listener, client, server := connectedPipePair(t)
	defer listener.Close()
	defer client.Close()
	defer server.Close()

	if err := client.SetDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	var first map[string]string
	if err := client.Receive(&first); err == nil {
		t.Fatal("expected read timeout")
	}
	if err := client.SetDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := server.Send(map[string]string{"state": "recovered"}); err != nil {
		t.Fatal(err)
	}
	var second map[string]string
	if err := client.Receive(&second); err != nil {
		t.Fatalf("connection did not recover after cancelled read: %v", err)
	}
	if second["state"] != "recovered" {
		t.Fatalf("message = %#v", second)
	}
}

func TestNamedPipeCloseCancelsPendingRead(t *testing.T) {
	listener, client, server := connectedPipePair(t)
	defer listener.Close()
	defer server.Close()

	readDone := make(chan error, 1)
	go func() {
		var value map[string]string
		readDone <- client.Receive(&value)
	}()
	time.Sleep(25 * time.Millisecond)
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-readDone:
		if err == nil {
			t.Fatal("pending read succeeded after close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pending read was not cancelled by close")
	}
}

func connectedPipePair(t *testing.T) (*NamedPipeListener, *Channel, *Channel) {
	t.Helper()
	name := fmt.Sprintf("Navo.IOTest.%d.%d", os.Getpid(), time.Now().UnixNano())
	listener, err := NewListener(name)
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan *Channel, 1)
	acceptErr := make(chan error, 1)
	go func() {
		server, err := listener.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- server
	}()
	client, err := Dial(name)
	if err != nil {
		listener.Close()
		t.Fatal(err)
	}
	select {
	case server := <-accepted:
		return listener, client, server
	case err := <-acceptErr:
		client.Close()
		listener.Close()
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		client.Close()
		listener.Close()
		t.Fatal("accept timed out")
	}
	return nil, nil, nil
}
