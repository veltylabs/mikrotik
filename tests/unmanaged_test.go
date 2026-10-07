package tests

import (
	"testing"

	"github.com/veltylabs/mikrotik/routeros"
	"github.com/veltylabs/mikrotik/v6"
	"webtyp.com/network"
)

func TestUnmanaged(t *testing.T) {
	emu := newEmulator("6.49.19")
	gw := v6.New(emu)

	// Pre-existing unmanaged forward rule and lease
	emu.tables["/ip/dhcp-server/lease"].records = append(emu.tables["/ip/dhcp-server/lease"].records, routeros.Record{
		".id":         "*A",
		"mac-address": "FF:EE:DD:CC:BB:AA",
		"address":     "10.0.0.99",
		"dynamic":     "false",
		"server":      "dhcp1",
	})
	emu.tables["/ip/firewall/filter"].records = append(emu.tables["/ip/firewall/filter"].records, routeros.Record{
		".id":             "*B",
		"action":          "accept",
		"src-mac-address": "FF:EE:DD:CC:BB:AA",
		"disabled":        "false",
	})

	desired := network.Desired{
		Settings: network.Settings{
			DHCPServer:   "dhcp1",
			DynamicPool:  "pool1",
			FilterDNS:    "1.1.1.3",
			Unregistered: network.UnregisteredNoAddress,
		},
		Hosts: []network.Host{
			{Name: "h1", MAC: "AA:BB:CC:DD:EE:11", IP: "10.0.0.1", Access: network.AccessInternetFiltered},
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

	// Byte-identical
	leaseFound := false
	for _, l := range emu.tables["/ip/dhcp-server/lease"].records {
		if l[".id"] == "*A" {
			leaseFound = true
			if l["address"] != "10.0.0.99" || l["mac-address"] != "FF:EE:DD:CC:BB:AA" || l["dynamic"] != "false" {
				t.Errorf("unmanaged lease modified")
			}
		}
	}
	if !leaseFound {
		t.Errorf("unmanaged lease deleted")
	}

	ruleFound := false
	for _, f := range emu.tables["/ip/firewall/filter"].records {
		if f[".id"] == "*B" {
			ruleFound = true
			if f["action"] != "accept" || f["src-mac-address"] != "FF:EE:DD:CC:BB:AA" {
				t.Errorf("unmanaged rule modified")
			}
		}
	}
	if !ruleFound {
		t.Errorf("unmanaged rule deleted")
	}

	// Assert warnings
	foundWarning := false
	for _, w := range plan.Warnings {
		if w.MAC == "FF:EE:DD:CC:BB:AA" && w.Reason == "has Internet by a hand-made rule but is not registered" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected warning for unmanaged rule")
	}
}
