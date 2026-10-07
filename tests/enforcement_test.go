package tests

import (
	"testing"

	"github.com/veltylabs/mikrotik/v6"
	"webtyp.com/network"
)

func TestEnforcement(t *testing.T) {
	emu := newEmulator("6.49.19")
	gw := v6.New(emu)

	desired := network.Desired{
		Settings: network.Settings{
			DHCPServer:   "dhcp1",
			DynamicPool:  "pool1",
			FilterDNS:    "1.1.1.3",
			Unregistered: network.UnregisteredNoAddress,
		},
		Hosts: []network.Host{
			{Name: "h1", MAC: "AA:BB:CC:DD:EE:11", IP: "10.0.0.1", Access: network.AccessInternetFiltered},
			{Name: "h2", MAC: "AA:BB:CC:DD:EE:22", IP: "10.0.0.2", Access: network.AccessInternet},
		},
	}

	plan, err := gw.Plan(desired)
	if err != nil {
		t.Fatal(err)
	}

	_, err = gw.Apply(desired, plan.Fingerprint)
	if err != nil {
		t.Fatal(err)
	}

	// 2 leases
	if len(emu.tables["/ip/dhcp-server/lease"].records) != 2 {
		t.Fatalf("expected 2 leases, got %d", len(emu.tables["/ip/dhcp-server/lease"].records))
	}

	// 3 forward rules at top
	filters := emu.tables["/ip/firewall/filter"].records
	if len(filters) != 3 {
		t.Fatalf("expected 3 filter rules, got %d", len(filters))
	}
	if filters[0]["comment"] != v6.BaselineInternet {
		t.Errorf("expected top rule to be internet")
	}

	// 2 dstnat rules with to-addresses=1.1.1.3
	nats := emu.tables["/ip/firewall/nat"].records
	if len(nats) != 2 {
		t.Fatalf("expected 2 nat rules, got %d", len(nats))
	}
	if nats[0]["to-addresses"] != "1.1.1.3" {
		t.Errorf("expected nat to-addresses=1.1.1.3")
	}

	// DHCP server static-only, add-arp=yes
	dhcp := emu.tables["/ip/dhcp-server"].records[0]
	if dhcp["address-pool"] != "static-only" || dhcp["add-arp"] != "yes" {
		t.Errorf("expected dhcp server to have static-only and add-arp=yes")
	}

	// bridge reply-only
	bridge := emu.tables["/interface/bridge"].records[0]
	if bridge["arp"] != "reply-only" {
		t.Errorf("expected bridge arp=reply-only")
	}

	// switch to UnregisteredLocal
	desired.Settings.Unregistered = network.UnregisteredLocal
	plan2, err := gw.Plan(desired)
	if err != nil {
		t.Fatal(err)
	}
	_, err = gw.Apply(desired, plan2.Fingerprint)
	if err != nil {
		t.Fatal(err)
	}

	dhcp2 := emu.tables["/ip/dhcp-server"].records[0]
	if dhcp2["address-pool"] != "pool1" || dhcp2["add-arp"] != "no" {
		t.Errorf("expected dhcp server to have pool1 and add-arp=no")
	}
	bridge2 := emu.tables["/interface/bridge"].records[0]
	if bridge2["arp"] != "enabled" {
		t.Errorf("expected bridge arp=enabled")
	}
}
