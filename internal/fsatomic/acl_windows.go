//go:build windows

package fsatomic

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func restrictPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	var token windows.Token
	if err := windows.OpenProcessToken(
		windows.CurrentProcess(), windows.TOKEN_QUERY, &token,
	); err != nil {
		return fmt.Errorf("open process token: %w", err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("read process user: %w", err)
	}
	sid := user.User.Sid.String()
	if sid == "" {
		return fmt.Errorf("process user SID is empty")
	}
	inheritance := ""
	if info.IsDir() {
		inheritance = "OICI"
	}
	descriptor, err := windows.SecurityDescriptorFromString(fmt.Sprintf(
		"D:P(A;%s;FA;;;SY)(A;%s;FA;;;BA)(A;%s;FA;;;%s)",
		inheritance, inheritance, inheritance, sid,
	))
	if err != nil {
		return fmt.Errorf("build protected DACL: %w", err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return fmt.Errorf("read protected DACL: %w", err)
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|
			windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	)
}
