//go:build routeros_integration

package tests

import (
	"strings"
	"testing"

	"github.com/veltylabs/mikrotik"
	"webtyp.com/network"
)

type intFixture struct {
	gw *mikrotik.Gateway
}

func (f *intFixture) Gateway() network.Gateway {
	return f.gw
}

func (f *intFixture) Settings() network.Settings {
	return network.Settings{
		DHCPServer:   "velty-test",
		DynamicPool:  "velty-test-pool",
		FilterDNS:    "1.1.1.3",
		Unregistered: network.UnregisteredNoAddress,
	}
}

func (f *intFixture) AddUnmanaged(found network.Discovered) error {
	panic("unimplemented AddUnmanaged for integration test yet")
}

func TestIntegrationV6(t *testing.T) {
	gw, err := mikrotik.Open("routeros://admin:@127.0.0.1:18728")
	if err != nil {
		t.Fatalf("failed to open ros6: %v", err)
	}
	defer gw.Close()

	v, err := gw.Version()
	if err != nil {
		t.Fatalf("failed to get version: %v", err)
	}
	if !strings.HasPrefix(v, "6.") {
		t.Fatalf("expected version 6.x, got %s", v)
	}
}

func TestIntegrationV7(t *testing.T) {
	gw, err := mikrotik.Open("routeros://admin:@127.0.0.1:28728")
	if err != nil {
		t.Fatalf("failed to open ros7: %v", err)
	}
	defer gw.Close()

	v, err := gw.Version()
	if err != nil {
		t.Fatalf("failed to get version: %v", err)
	}
	if !strings.HasPrefix(v, "7.") {
		t.Fatalf("expected version 7.x, got %s", v)
	}
}
