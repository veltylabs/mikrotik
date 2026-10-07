package v6

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/veltylabs/mikrotik/routeros"
	"webtyp.com/network"
)

type Action struct {
	network.Change
	Run []string
}

// planResult is our internal representation that pairs the network.Plan with the commands.
type planResult struct {
	network.Plan
	Actions []Action
}

func (g *Gateway) makePlan(d network.Desired) (planResult, error) {
	var pr planResult
	if err := d.Validate(); err != nil {
		return pr, err
	}

	snap, err := g.read(d.Settings)
	if err != nil {
		return pr, err
	}

	// 2. Settings (DHCP server and bridge)
	var dhcp routeros.Record
	for _, s := range snap.DHCPServers {
		if s["name"] == d.Settings.DHCPServer {
			dhcp = s
			break
		}
	}

	desiredPool := d.Settings.DynamicPool
	desiredAddARP := "no"
	desiredBridgeARP := "enabled"

	if d.Settings.Unregistered == network.UnregisteredNoAddress {
		desiredPool = StaticOnlyPool
		desiredAddARP = "yes"
		desiredBridgeARP = "reply-only"
	}

	if dhcp["address-pool"] != desiredPool || dhcp["add-arp"] != desiredAddARP {
		pr.Actions = append(pr.Actions, Action{
			Change: network.Change{
				Kind:   network.ChangeUpdate,
				Object: "dhcp-server " + d.Settings.DHCPServer,
			},
			Run: []string{PathDHCPServer + CmdSet, "=" + PropID + "=" + dhcp[PropID], "=address-pool=" + desiredPool, "=add-arp=" + desiredAddARP},
		})
	}

	bridgeName := dhcp["interface"]
	var bridge routeros.Record
	for _, b := range snap.Bridges {
		if b["name"] == bridgeName {
			bridge = b
			break
		}
	}

	if bridge != nil && bridge["arp"] != desiredBridgeARP {
		pr.Actions = append(pr.Actions, Action{
			Change: network.Change{
				Kind:   network.ChangeUpdate,
				Object: "bridge " + bridgeName,
			},
			Run: []string{PathBridge + CmdSet, "=" + PropID + "=" + bridge[PropID], "=arp=" + desiredBridgeARP},
		})
	}

	// 3. Baseline rules
	var intRule, intFiltRule, blockRule routeros.Record
	for _, f := range snap.Filters {
		switch f[PropComment] {
		case BaselineInternet:
			intRule = f
		case BaselineInternetFiltered:
			intFiltRule = f
		case BaselineBlock:
			blockRule = f
		}
	}

	checkFilter := func(existing routeros.Record, comment string, runArgs []string) {
		if existing == nil {
			args := append([]string{PathFilter + CmdAdd, "=" + PropComment + "=" + comment}, runArgs...)
			pr.Actions = append(pr.Actions, Action{
				Change: network.Change{
					Kind:   network.ChangeAdd,
					Object: comment,
				},
				Run: args,
			})
		} else {
			needsUpdate := false
			for _, arg := range runArgs {
				kv := strings.SplitN(strings.TrimPrefix(arg, "="), "=", 2)
				if len(kv) == 2 && existing[kv[0]] != kv[1] {
					needsUpdate = true
					break
				}
			}
			if needsUpdate {
				args := append([]string{PathFilter + CmdSet, "=" + PropID + "=" + existing[PropID]}, runArgs...)
				pr.Actions = append(pr.Actions, Action{
					Change: network.Change{
						Kind:   network.ChangeUpdate,
						Object: comment,
					},
					Run: args,
				})
			}
		}
	}

	checkFilter(intRule, BaselineInternet, []string{"=chain=forward", "=action=accept", "=src-address-list=" + ListInternet, "=out-interface-list=" + WANList})
	checkFilter(intFiltRule, BaselineInternetFiltered, []string{"=chain=forward", "=action=accept", "=src-address-list=" + ListInternetFiltered, "=out-interface-list=" + WANList})
	checkFilter(blockRule, BaselineBlock, []string{"=chain=forward", "=action=drop", "=in-interface=" + bridgeName, "=out-interface-list=" + WANList})

	var dnsUDP, dnsTCP routeros.Record
	for _, n := range snap.NATs {
		switch n[PropComment] {
		case BaselineDNSFilterUDP:
			dnsUDP = n
		case BaselineDNSFilterTCP:
			dnsTCP = n
		}
	}

	checkNAT := func(existing routeros.Record, comment string, runArgs []string) {
		if existing == nil {
			args := append([]string{PathNAT + CmdAdd, "=" + PropComment + "=" + comment}, runArgs...)
			pr.Actions = append(pr.Actions, Action{
				Change: network.Change{
					Kind:   network.ChangeAdd,
					Object: comment,
				},
				Run: args,
			})
		} else {
			needsUpdate := false
			for _, arg := range runArgs {
				kv := strings.SplitN(strings.TrimPrefix(arg, "="), "=", 2)
				if len(kv) == 2 && existing[kv[0]] != kv[1] {
					needsUpdate = true
					break
				}
			}
			if needsUpdate {
				args := append([]string{PathNAT + CmdSet, "=" + PropID + "=" + existing[PropID]}, runArgs...)
				pr.Actions = append(pr.Actions, Action{
					Change: network.Change{
						Kind:   network.ChangeUpdate,
						Object: comment,
					},
					Run: args,
				})
			}
		}
	}

	checkNAT(dnsUDP, BaselineDNSFilterUDP, []string{"=chain=dstnat", "=action=dst-nat", "=protocol=udp", "=dst-port=53", "=src-address-list=" + ListInternetFiltered, "=to-addresses=" + d.Settings.FilterDNS})
	checkNAT(dnsTCP, BaselineDNSFilterTCP, []string{"=chain=dstnat", "=action=dst-nat", "=protocol=tcp", "=dst-port=53", "=src-address-list=" + ListInternetFiltered, "=to-addresses=" + d.Settings.FilterDNS})

	// 4. Hosts
	desiredMACs := make(map[string]bool)
	for _, h := range d.Hosts {
		macUpper := strings.ToUpper(h.MAC)
		desiredMACs[macUpper] = true

		var existing routeros.Record
		for _, l := range snap.Leases {
			if strings.ToUpper(l[PropMAC]) == macUpper {
				existing = l
				break
			}
		}

		comment := ManagedMarker + " " + h.Name
		accessList := accessToList(h.Access)

		ip := h.IP
		if ip == "" {
			ip = "0.0.0.0"
		}

		if existing != nil {
			if isUnmanagedLease(existing) {
				if existing[PropAddress] != h.IP {
					pr.Conflicts = append(pr.Conflicts, network.Conflict{
						Host:   h,
						Reason: fmt.Sprintf("IP %s is held by unmanaged lease of %s", h.IP, macUpper),
					})
					continue
				}
				pr.Actions = append(pr.Actions, Action{
					Change: network.Change{
						Kind:   network.ChangeAdopt,
						Object: "dhcp lease " + h.IP,
						Host:   h,
					},
					Run: []string{PathLease + CmdSet, "=" + PropID + "=" + existing[PropID], "=" + PropComment + "=" + comment, "=" + PropAddressLists + "=" + accessList, "=" + PropServer + "=" + d.Settings.DHCPServer, "=" + PropAddress + "=" + h.IP},
				})
			} else if isManaged(existing[PropComment]) {
				if existing[PropAddress] != h.IP || existing[PropAddressLists] != accessList || existing[PropComment] != comment {
					pr.Actions = append(pr.Actions, Action{
						Change: network.Change{
							Kind:   network.ChangeUpdate,
							Object: "dhcp lease " + h.IP,
							Host:   h,
						},
						Run: []string{PathLease + CmdSet, "=" + PropID + "=" + existing[PropID], "=" + PropComment + "=" + comment, "=" + PropAddressLists + "=" + accessList, "=" + PropServer + "=" + d.Settings.DHCPServer, "=" + PropAddress + "=" + h.IP},
					})
				}
			} else {
				pr.Actions = append(pr.Actions, Action{
					Change: network.Change{
						Kind:   network.ChangeAdd,
						Object: "dhcp lease " + h.IP,
						Host:   h,
					},
					Run: []string{PathLease + CmdAdd, "=" + PropMAC + "=" + h.MAC, "=" + PropAddress + "=" + h.IP, "=" + PropServer + "=" + d.Settings.DHCPServer, "=" + PropComment + "=" + comment, "=" + PropAddressLists + "=" + accessList},
				})
			}
		} else {
			pr.Actions = append(pr.Actions, Action{
				Change: network.Change{
					Kind:   network.ChangeAdd,
					Object: "dhcp lease " + h.IP,
					Host:   h,
				},
				Run: []string{PathLease + CmdAdd, "=" + PropMAC + "=" + h.MAC, "=" + PropAddress + "=" + h.IP, "=" + PropServer + "=" + d.Settings.DHCPServer, "=" + PropComment + "=" + comment, "=" + PropAddressLists + "=" + accessList},
			})
		}
	}

	for _, h := range d.Hosts {
		macUpper := strings.ToUpper(h.MAC)
		for _, l := range snap.Leases {
			if l[PropAddress] == h.IP && strings.ToUpper(l[PropMAC]) != macUpper {
				if isUnmanagedLease(l) {
					pr.Conflicts = append(pr.Conflicts, network.Conflict{
						Host:   h,
						Reason: fmt.Sprintf("IP %s is held by unmanaged lease of %s", h.IP, strings.ToUpper(l[PropMAC])),
					})
				}
			}
		}
	}

	for _, l := range snap.Leases {
		macUpper := strings.ToUpper(l[PropMAC])
		if isManaged(l[PropComment]) && !desiredMACs[macUpper] {

			hostName := strings.TrimSpace(strings.TrimPrefix(l[PropComment], ManagedMarker))

			host := network.Host{
				MAC:    macUpper,
				IP:     l[PropAddress],
				Name:   hostName,
				Access: listToAccess(l[PropAddressLists]),
			}

			pr.Actions = append(pr.Actions, Action{
				Change: network.Change{
					Kind:   network.ChangeRemove,
					Object: "dhcp lease " + l[PropAddress],
					Host:   host,
				},
				Run: []string{PathLease + CmdRemove, "=" + PropID + "=" + l[PropID]},
			})
		}
	}

	for _, f := range snap.Filters {
		if isUnmanagedInternetRule(f) {
			macUpper := strings.ToUpper(f["src-mac-address"])
			if !desiredMACs[macUpper] {
				pr.Warnings = append(pr.Warnings, network.Warning{
					MAC:    macUpper,
					Reason: "has Internet by a hand-made rule but is not registered",
				})
			}
		}
	}

	sort.Slice(pr.Conflicts, func(i, j int) bool { return pr.Conflicts[i].Host.MAC < pr.Conflicts[j].Host.MAC })
	sort.Slice(pr.Warnings, func(i, j int) bool { return pr.Warnings[i].MAC < pr.Warnings[j].MAC })

	hasher := sha256.New()
	writeRecords := func(path string, recs []routeros.Record) {
		sort.Slice(recs, func(i, j int) bool { return recs[i][PropID] < recs[j][PropID] })
		for _, r := range recs {
			hasher.Write([]byte(path + "\n"))
			var keys []string
			for k := range r {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				hasher.Write([]byte(k + "=" + r[k] + "\n"))
			}
		}
	}

	writeRecords(PathARP, snap.ARPs)
	writeRecords(PathBridge, snap.Bridges)
	writeRecords(PathDHCPServer, snap.DHCPServers)
	writeRecords(PathFilter, snap.Filters)
	writeRecords(PathIfaceList, snap.IfaceLists)
	writeRecords(PathLease, snap.Leases)
	writeRecords(PathNAT, snap.NATs)

	for _, c := range pr.Actions {
		pr.Changes = append(pr.Changes, c.Change)
		hasher.Write([]byte(string(c.Change.Kind) + " " + c.Change.Object + "\n"))
		for _, arg := range c.Run {
			hasher.Write([]byte(arg + "\n"))
		}
	}

	pr.Fingerprint = network.Fingerprint(hex.EncodeToString(hasher.Sum(nil)))

	return pr, nil
}

func (g *Gateway) Plan(d network.Desired) (network.Plan, error) {
	pr, err := g.makePlan(d)
	return pr.Plan, err
}
