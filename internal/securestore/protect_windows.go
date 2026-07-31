//go:build windows

package securestore

import (
	"fmt"
	"syscall"
	"unsafe"
)

const cryptProtectLocalMachine = 0x4

type dataBlob struct {
	size uint32
	data *byte
}

var (
	crypt32            = syscall.NewLazyDLL("crypt32.dll")
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	cryptProtectData   = crypt32.NewProc("CryptProtectData")
	cryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	localFree          = kernel32.NewProc("LocalFree")
)

func blob(data []byte) dataBlob {
	if len(data) == 0 {
		return dataBlob{}
	}
	return dataBlob{size: uint32(len(data)), data: &data[0]}
}

// Protect encrypts data with Windows DPAPI machine scope so the service can read it.
func Protect(plain []byte) ([]byte, error) {
	input := blob(plain)
	var output dataBlob
	ok, _, callErr := cryptProtectData.Call(
		uintptr(unsafe.Pointer(&input)),
		0, 0, 0, 0,
		cryptProtectLocalMachine,
		uintptr(unsafe.Pointer(&output)),
	)
	if ok == 0 {
		return nil, fmt.Errorf("CryptProtectData: %w", callErr)
	}
	defer localFree.Call(uintptr(unsafe.Pointer(output.data)))
	return append([]byte(nil), unsafe.Slice(output.data, output.size)...), nil
}

// Unprotect decrypts data previously protected by Protect.
func Unprotect(ciphertext []byte) ([]byte, error) {
	input := blob(ciphertext)
	var output dataBlob
	ok, _, callErr := cryptUnprotectData.Call(
		uintptr(unsafe.Pointer(&input)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&output)),
	)
	if ok == 0 {
		return nil, fmt.Errorf("CryptUnprotectData: %w", callErr)
	}
	defer localFree.Call(uintptr(unsafe.Pointer(output.data)))
	return append([]byte(nil), unsafe.Slice(output.data, output.size)...), nil
}
