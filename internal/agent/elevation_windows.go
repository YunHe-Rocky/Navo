package agent

import "golang.org/x/sys/windows"

func processIsElevated() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(
		windows.CurrentProcess(),
		windows.TOKEN_QUERY,
		&token,
	); err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}
