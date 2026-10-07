package tests

import (
	"fmt"
	"testing"

	"github.com/veltylabs/mikrotik/routeros"
	"github.com/veltylabs/mikrotik/v6"
	"github.com/veltylabs/mikrotik/v7"
	"webtyp.com/network"
	"webtyp.com/network/conformance"
)

type fixture struct {
	emu *emulator
	gw  network.Gateway
}

func (f *fixture) Gateway() network.Gateway {
	return f.gw
}

func (f *fixture) Settings() network.Settings {
	return network.Settings{
		DHCPServer:   "dhcp1",
		DynamicPool:  "pool1",
		FilterDNS:    "1.1.1.3",
		Unregistered: network.UnregisteredNoAddress,
	}
}

func (f *fixture) AddUnmanaged(found network.Discovered) error {
	lease := routeros.Record{
		".id":         fmt.Sprintf("*%d", f.emu.tables["/ip/dhcp-server/lease"].idSeq+1),
		"mac-address": found.MAC,
		"address":     found.IP,
		"dynamic":     "false",
		"server":      "dhcp1",
	}
	if found.Name != "" {
		lease["comment"] = found.Name
	}
	f.emu.tables["/ip/dhcp-server/lease"].idSeq++
	f.emu.tables["/ip/dhcp-server/lease"].records = append(f.emu.tables["/ip/dhcp-server/lease"].records, lease)

	if found.Internet {
		rule := routeros.Record{
			".id":             fmt.Sprintf("*%d", f.emu.tables["/ip/firewall/filter"].idSeq+1),
			"action":          "accept",
			"src-mac-address": found.MAC,
			"disabled":        "false",
		}
		if found.Name != "" {
			rule["comment"] = found.Name
		}
		f.emu.tables["/ip/firewall/filter"].idSeq++
		f.emu.tables["/ip/firewall/filter"].records = append(f.emu.tables["/ip/firewall/filter"].records, rule)
	}
	return nil
}

func TestConformanceV6(t *testing.T) {
	conformance.Run(t, func(t *testing.T) conformance.Fixture {
		emu := newEmulator("6.49.19")
		gw := v6.New(emu)
		return &fixture{emu: emu, gw: gw}
	})
}

func TestConformanceV7(t *testing.T) {
	conformance.Run(t, func(t *testing.T) conformance.Fixture {
		emu := newEmulator("7.20.2")
		gw := v7.New(emu)
		return &fixture{emu: emu, gw: gw}
	})
}
