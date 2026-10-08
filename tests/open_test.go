package tests

import (
	"testing"

	"github.com/veltylabs/mikrotik"
	"webtyp.com/network"
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

func TestOpenNoDial(t *testing.T) {
	// 127.0.0.1:1 is guaranteed to be closed; if Open dials, this will block and error.
	gw, err := mikrotik.Open("routeros://admin:x@127.0.0.1:1")
	if err != nil {
		t.Fatalf("expected Open to not dial, got error: %v", err)
	}

	// Dialing is deferred until a command like Plan is executed.
	_, err = gw.Plan(network.Desired{})
	if err == nil {
		t.Errorf("expected error on deferred dial, got nil")
	}
}
