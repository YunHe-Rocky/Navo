//go:build windows

package main

import (
	"fmt"
	"os"
	"testing"
)

func TestNamedMutexEnforcesSingleOwnerAndReleases(t *testing.T) {
	name := fmt.Sprintf(`Local\Navo.Test.%d`, os.Getpid())
	first, already, err := acquireNamedInstance(name)
	if err != nil || already {
		t.Fatalf("first acquire = lock:%v already:%t err:%v", first, already, err)
	}
	defer first.Close()

	second, already, err := acquireNamedInstance(name)
	if err != nil || !already || second != nil {
		t.Fatalf("second acquire = lock:%v already:%t err:%v", second, already, err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	third, already, err := acquireNamedInstance(name)
	if err != nil || already {
		t.Fatalf("reacquire = lock:%v already:%t err:%v", third, already, err)
	}
	if err := third.Close(); err != nil {
		t.Fatal(err)
	}
}
