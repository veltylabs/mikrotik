package tests

import (
	"testing"

	"github.com/veltylabs/mikrotik/routeros"
	"github.com/veltylabs/mikrotik/v6"
	"webtyp.com/network"
)

func TestConnections(t *testing.T) {
	emu := newEmulator("6.49.19")
	gw := v6.New(emu)

	emu.tables["/ip/dhcp-server/lease"].records = append(emu.tables["/ip/dhcp-server/lease"].records, routeros.Record{
		".id":            "*C",
		"mac-address":    "11:22:33:44:55:66",
		"address":        "10.0.0.100",
		"active-address": "10.0.0.100",
		"status":         "bound",
		"host-name":      "my-pc",
	})

	emu.tables["/ip/arp"].records = append(emu.tables["/ip/arp"].records, routeros.Record{
		".id":         "*D",
		"mac-address": "66:55:44:33:22:11",
		"address":     "10.0.0.101",
		"complete":    "true",
	})

	// ARP for the bound lease should not emit SourceARP
	emu.tables["/ip/arp"].records = append(emu.tables["/ip/arp"].records, routeros.Record{
		".id":         "*E",
		"mac-address": "11:22:33:44:55:66",
		"address":     "10.0.0.100",
		"complete":    "true",
	})

	conns, err := gw.Connections()
	if err != nil {
		t.Fatal(err)
	}

	if len(conns) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(conns))
	}

	foundDHCP := false
	foundARP := false

	for _, c := range conns {
		if c.MAC == "11:22:33:44:55:66" {
			if c.Source != network.SourceDHCP {
				t.Errorf("expected SourceDHCP for bound lease")
			}
			if c.HostName != "my-pc" {
				t.Errorf("expected HostName to be my-pc")
			}
			foundDHCP = true
		}
		if c.MAC == "66:55:44:33:22:11" {
			if c.Source != network.SourceARP {
				t.Errorf("expected SourceARP for arp-only")
			}
			foundARP = true
		}
	}

	if !foundDHCP || !foundARP {
		t.Errorf("missing expected connections")
	}
}
