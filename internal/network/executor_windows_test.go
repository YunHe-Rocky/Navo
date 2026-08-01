//go:build windows

package network

import (
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func TestDecodeCommandOutputFallsBackToGBK(t *testing.T) {
	encoded, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), []byte("路由失败"))
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeCommandOutput(encoded); got != "路由失败" {
		t.Fatalf("decoded output = %q", got)
	}
}
