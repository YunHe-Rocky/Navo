//go:build windows

package fsatomic

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// RepairTree restores exact access for the process identity and profile owners
// after an older elevated build may have protected paths for only one SID.
// WebView2 owns a large disposable cache tree and is intentionally excluded.
func RepairTree(root string) error {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := restrictPath(root); err != nil {
		return fmt.Errorf("repair profile root ACL: %w", err)
	}
	var result error
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			result = errors.Join(result, fmt.Errorf("walk %s: %w", path, walkErr))
			return nil
		}
		if path == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() && strings.EqualFold(entry.Name(), "WebView2") {
			return filepath.SkipDir
		}
		if err := restrictPath(path); err != nil {
			result = errors.Join(result, fmt.Errorf("repair ACL %s: %w", path, err))
		}
		return nil
	})
	return errors.Join(result, err)
}

func restrictPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	processSID, err := processUserSID()
	if err != nil {
		return err
	}
	// An elevated process can run with a different credential while retaining
	// the interactive user's LOCALAPPDATA path. Preserve user owners found on
	// the path ancestry so one atomic write cannot lock that profile out.
	allowedSIDs := append([]string{processSID}, pathOwnerSIDs(path)...)
	descriptor, err := protectedDACLDescriptor(info.IsDir(), allowedSIDs)
	if err != nil {
		return err
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

func processUserSID() (string, error) {
	var token windows.Token
	if err := windows.OpenProcessToken(
		windows.CurrentProcess(), windows.TOKEN_QUERY, &token,
	); err != nil {
		return "", fmt.Errorf("open process token: %w", err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("read process user: %w", err)
	}
	sid := user.User.Sid.String()
	if sid == "" {
		return "", fmt.Errorf("process user SID is empty")
	}
	return sid, nil
}

func pathOwnerSIDs(path string) []string {
	seen := make(map[string]struct{})
	var result []string
	current := filepath.Clean(path)
	for {
		descriptor, err := windows.GetNamedSecurityInfo(
			current, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION,
		)
		if err == nil {
			owner, _, ownerErr := descriptor.Owner()
			if ownerErr == nil && owner != nil {
				sid := owner.String()
				// Account/domain SIDs are safe exact principals. Built-in groups,
				// SYSTEM and broad identities are deliberately not propagated.
				if strings.HasPrefix(sid, "S-1-5-21-") {
					key := strings.ToUpper(sid)
					if _, exists := seen[key]; !exists {
						seen[key] = struct{}{}
						result = append(result, sid)
					}
				}
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return result
}

func protectedDACLDescriptor(directory bool, sids []string) (*windows.SECURITY_DESCRIPTOR, error) {
	inheritance := ""
	if directory {
		inheritance = "OICI"
	}
	var acl strings.Builder
	fmt.Fprintf(&acl, "D:P(A;%s;FA;;;SY)(A;%s;FA;;;BA)", inheritance, inheritance)
	seen := make(map[string]struct{})
	for _, sid := range sids {
		sid = strings.TrimSpace(sid)
		if sid == "" {
			continue
		}
		key := strings.ToUpper(sid)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		fmt.Fprintf(&acl, "(A;%s;FA;;;%s)", inheritance, sid)
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("protected DACL has no user SID")
	}
	descriptor, err := windows.SecurityDescriptorFromString(acl.String())
	if err != nil {
		return nil, fmt.Errorf("build protected DACL: %w", err)
	}
	return descriptor, nil
}
