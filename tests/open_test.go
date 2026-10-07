package tests

import (
	"testing"

	"github.com/veltylabs/mikrotik"
)

func TestOpenErrors(t *testing.T) {
	_, err := mikrotik.Open("http://admin:@127.0.0.1:8728")
	if err != mikrotik.ErrScheme {
		t.Errorf("expected ErrScheme, got %v", err)
	}

	_, err = mikrotik.Open("routeros://127.0.0.1:8728")
	if err == nil || err.Error() != "mikrotik: ROUTER_URL has no user" {
		t.Errorf("expected no user error, got %v", err)
	}
}
