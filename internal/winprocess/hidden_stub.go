//go:build !windows

package winprocess

import "os/exec"

func ConfigureHidden(_ *exec.Cmd) {}
