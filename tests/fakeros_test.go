package tests

import (
	"fmt"
	"strings"

	"github.com/veltylabs/mikrotik/routeros"
)

type table struct {
	records []routeros.Record
	idSeq   int
}

type emulator struct {
	tables map[string]*table
}

func newEmulator(version string) *emulator {
	e := &emulator{
		tables: make(map[string]*table),
	}

	e.tables["/system/resource"] = &table{
		records: []routeros.Record{
			{"version": version},
		},
	}

	e.tables["/ip/dhcp-server"] = &table{
		records: []routeros.Record{
			{".id": "*1", "name": "dhcp1", "interface": "br1", "address-pool": "pool1"},
		},
	}

	e.tables["/interface/bridge"] = &table{
		records: []routeros.Record{
			{".id": "*1", "name": "br1", "arp": "enabled"},
		},
	}

	e.tables["/interface/list"] = &table{
		records: []routeros.Record{
			{".id": "*1", "name": "WAN"},
		},
	}

	e.tables["/ip/dhcp-server/lease"] = &table{}
	e.tables["/ip/firewall/filter"] = &table{}
	e.tables["/ip/firewall/nat"] = &table{}
	e.tables["/ip/arp"] = &table{}

	return e
}

func (e *emulator) Run(sentence ...string) (routeros.Reply, error) {
	cmd := sentence[0]

	if cmd == "/system/resource/print" {
		return routeros.Reply{Records: e.tables["/system/resource"].records}, nil
	}

	path := cmd[:strings.LastIndex(cmd, "/")]
	verb := cmd[strings.LastIndex(cmd, "/")+1:]

	t, ok := e.tables[path]
	if !ok {
		return routeros.Reply{}, fmt.Errorf("unknown path %s", path)
	}

	switch verb {
	case "print":
		return routeros.Reply{Records: t.records}, nil
	case "add":
		t.idSeq++
		id := fmt.Sprintf("*%x", t.idSeq) // use hex like mikrotik

		rec := routeros.Record{".id": id}
		var placeBefore string
		for _, arg := range sentence[1:] {
			if strings.HasPrefix(arg, "=") {
				kv := strings.SplitN(strings.TrimPrefix(arg, "="), "=", 2)
				if len(kv) == 2 {
					if kv[0] == "place-before" {
						placeBefore = kv[1]
					} else {
						rec[kv[0]] = kv[1]
					}
				}
			}
		}

		if placeBefore != "" {
			idx := -1
			for i, r := range t.records {
				if r[".id"] == placeBefore {
					idx = i
					break
				}
			}
			if idx != -1 {
				t.records = append(t.records[:idx], append([]routeros.Record{rec}, t.records[idx:]...)...)
			} else {
				t.records = append(t.records, rec)
			}
		} else {
			t.records = append(t.records, rec)
		}

		return routeros.Reply{Done: routeros.Record{"ret": id}}, nil

	case "set":
		var id string
		args := make(map[string]string)
		for _, arg := range sentence[1:] {
			if strings.HasPrefix(arg, "=") {
				kv := strings.SplitN(strings.TrimPrefix(arg, "="), "=", 2)
				if len(kv) == 2 {
					if kv[0] == ".id" {
						id = kv[1]
					} else {
						args[kv[0]] = kv[1]
					}
				}
			}
		}

		for i, r := range t.records {
			if r[".id"] == id {
				for k, v := range args {
					t.records[i][k] = v
				}
				return routeros.Reply{}, nil
			}
		}
		return routeros.Reply{}, fmt.Errorf("no such item")

	case "remove":
		var id string
		for _, arg := range sentence[1:] {
			if strings.HasPrefix(arg, "=.id=") {
				id = strings.TrimPrefix(arg, "=.id=")
				break
			}
		}

		for i, r := range t.records {
			if r[".id"] == id {
				t.records = append(t.records[:i], t.records[i+1:]...)
				return routeros.Reply{}, nil
			}
		}
		return routeros.Reply{}, fmt.Errorf("no such item")

	default:
		return routeros.Reply{}, fmt.Errorf("unknown verb %s", verb)
	}
}
