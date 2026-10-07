package v6

import (
	"fmt"
	"strings"

	"github.com/veltylabs/mikrotik/routeros"
	"webtyp.com/network"
)

type snapshot struct {
	DHCPServers []routeros.Record
	Bridges     []routeros.Record
	Leases      []routeros.Record
	Filters     []routeros.Record
	NATs        []routeros.Record
	ARPs        []routeros.Record
	IfaceLists  []routeros.Record
}

func (g *Gateway) read(settings network.Settings) (snapshot, error) {
	var snap snapshot

	// Read DHCP Servers
	r, err := g.C.Run(PathDHCPServer + CmdPrint)
	if err != nil {
		return snap, err
	}
	snap.DHCPServers = r.Records

	var dhcpFound bool
	for _, s := range snap.DHCPServers {
		if s["name"] == settings.DHCPServer {
			dhcpFound = true
			break
		}
	}
	if !dhcpFound {
		return snap, fmt.Errorf("mikrotik: DHCP server %q not found", settings.DHCPServer)
	}

	// Read Bridges
	r, err = g.C.Run(PathBridge + CmdPrint)
	if err != nil {
		return snap, err
	}
	snap.Bridges = r.Records

	// Read Interface Lists
	r, err = g.C.Run(PathIfaceList + CmdPrint)
	if err != nil {
		return snap, err
	}
	snap.IfaceLists = r.Records

	var wanFound bool
	for _, l := range snap.IfaceLists {
		if l["name"] == WANList {
			wanFound = true
			break
		}
	}
	if !wanFound {
		return snap, fmt.Errorf("mikrotik: interface list %q not found", WANList)
	}

	// Read Leases
	r, err = g.C.Run(PathLease + CmdPrint)
	if err != nil {
		return snap, err
	}
	snap.Leases = r.Records

	// Read Filters
	r, err = g.C.Run(PathFilter + CmdPrint)
	if err != nil {
		return snap, err
	}
	snap.Filters = r.Records

	// Read NATs
	r, err = g.C.Run(PathNAT + CmdPrint)
	if err != nil {
		return snap, err
	}
	snap.NATs = r.Records

	// Read ARPs
	r, err = g.C.Run(PathARP + CmdPrint)
	if err != nil {
		return snap, err
	}
	snap.ARPs = r.Records

	return snap, nil
}

func isManaged(comment string) bool {
	return strings.HasPrefix(comment, ManagedMarker)
}

func isUnmanagedLease(rec routeros.Record) bool {
	return rec[PropDynamic] == "false" && !isManaged(rec[PropComment])
}

func isUnmanagedInternetRule(rec routeros.Record) bool {
	return rec["action"] == "accept" && rec["src-mac-address"] != "" && rec["disabled"] != "true" && !isManaged(rec[PropComment])
}
