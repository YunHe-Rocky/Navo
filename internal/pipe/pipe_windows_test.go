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
