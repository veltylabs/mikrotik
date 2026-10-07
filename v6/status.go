package v6

import (
	"strings"

	"webtyp.com/network"
)

func (g *Gateway) Connections() ([]network.Connection, error) {
	return g.connections()
}

func (g *Gateway) Discover() ([]network.Discovered, error) {
	return g.discover()
}

func (g *Gateway) readAll() (snapshot, error) {
	var snap snapshot

	// Read Leases
	r, err := g.C.Run(PathLease + CmdPrint)
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

	// Read ARPs
	r, err = g.C.Run(PathARP + CmdPrint)
	if err != nil {
		return snap, err
	}
	snap.ARPs = r.Records

	return snap, nil
}

func (g *Gateway) connections() ([]network.Connection, error) {
	snap, err := g.readAll()
	if err != nil {
		return nil, err
	}

	var conns []network.Connection
	boundMACs := make(map[string]bool)

	for _, l := range snap.Leases {
		if l[PropStatus] == "bound" {
			mac := strings.ToUpper(l[PropMAC])
			boundMACs[mac] = true

			ip := l["active-address"]
			if ip == "" {
				ip = l[PropAddress]
			}

			conns = append(conns, network.Connection{
				MAC:    mac,
				IP:     ip,
				HostName: l[PropHostName],
				Source: network.SourceDHCP,
			})
		}
	}

	for _, arp := range snap.ARPs {
		if arp["complete"] == "true" {
			mac := strings.ToUpper(arp[PropMAC])
			if !boundMACs[mac] {
				conns = append(conns, network.Connection{
					MAC:    mac,
					IP:     arp[PropAddress],
					Source: network.SourceARP,
				})
			}
		}
	}

	return conns, nil
}

func (g *Gateway) discover() ([]network.Discovered, error) {
	snap, err := g.readAll()
	if err != nil {
		return nil, err
	}

	discoveredByMAC := make(map[string]*network.Discovered)

	for _, l := range snap.Leases {
		if isUnmanagedLease(l) {
			mac := strings.ToUpper(l[PropMAC])
			discoveredByMAC[mac] = &network.Discovered{
				MAC:  mac,
				IP:   l[PropAddress],
				Name: l[PropComment],
			}
		}
	}

	for _, f := range snap.Filters {
		if isUnmanagedInternetRule(f) {
			mac := strings.ToUpper(f["src-mac-address"])
			d, exists := discoveredByMAC[mac]
			if !exists {
				d = &network.Discovered{
					MAC:  mac,
					Name: f[PropComment],
				}
				discoveredByMAC[mac] = d
			}
			d.Internet = true
		}
	}

	var result []network.Discovered
	for _, d := range discoveredByMAC {
		result = append(result, *d)
	}

	return result, nil
}
