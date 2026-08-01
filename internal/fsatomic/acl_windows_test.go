//go:build windows

package fsatomic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWriteFileAppliesProtectedCurrentUserDACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "state.json")
	if err := WriteFile(path, []byte("state"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(path); err != nil {
		t.Fatal(err)
	}

	var token windows.Token
	if err := windows.OpenProcessToken(
		windows.CurrentProcess(), windows.TOKEN_QUERY, &token,
	); err != nil {
		t.Fatal(err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.GetNamedSecurityInfo(
		path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatal(err)
	}
	sddl := descriptor.String()
	if !strings.Contains(sddl, "D:P") || !strings.Contains(sddl, user.User.Sid.String()) {
		t.Fatalf("file DACL is not protected for current user: %q", sddl)
	}
	for _, broadSID := range []string{";;;WD)", ";;;BU)", ";;;AU)"} {
		if strings.Contains(sddl, broadSID) {
			t.Fatalf("file DACL grants broad access: %q", sddl)
		}
	}
}
