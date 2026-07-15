package main

import (
	"strings"
	"testing"

	"golang.org/x/term"
)

func TestPasswordCLIRequiresTTY(t *testing.T) {
	if term.IsTerminal(int(stdinFD())) {
		t.Skip("test runner stdin is a terminal")
	}
	if _, err := readPasswordPair(); err == nil || !strings.Contains(err.Error(), "interactive TTY") {
		t.Fatalf("err=%v", err)
	}
}
func stdinFD() uintptr { return 0 }
