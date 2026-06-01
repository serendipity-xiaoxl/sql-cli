package main

import "testing"

func TestBuild(t *testing.T) {
	// Verify the binary compiles correctly.
	// This is a compile-time check — the actual CLI is tested via integration.
	var _ = appVersion
}
