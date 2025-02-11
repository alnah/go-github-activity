package main

import (
	"testing"
)

func assertNoError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func assertNotNil(t testing.TB, err error) {
	t.Helper()
	if err == nil {
		t.Error("expected an error, but got nil")
	}
}
