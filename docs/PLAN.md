---
PLAN: "feat: MikroTik RouterOS gateway for webtyp.com/network (v6 and v7 dialects)"
TAG: v0.1.0
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
>
> Phase F3 of the network administration master plan (private repo `veltylabs/mjosefa-cms`; you do
> not need it). **Depends on `webtyp.com/network` v0.1.0** (phase F2). First line of work:
> `go get webtyp.com/network@latest`. If that version does not exist yet, STOP and report — never add
> a `replace`, never copy its types.

# Plan — `github.com/veltylabs/mikrotik`

Read first: [AGENTS.md](../AGENTS.md) and [docs/ARCHITECTURE.md](ARCHITECTURE.md). The architecture
doc's tables ("How access is enforced", "Apply order") are the spec for Stage 3; this plan does not
repeat them.

The contract you implement is `webtyp.com/network`: read its `plan.go`, `network.go`, `access.go`,
`errors.go`, and `conformance/conformance.go` in the module cache after `go get`
(`$(go env GOMODCACHE)/webtyp.com/network@v0.1.0/`). Its semantics (adopt, conflict, warning,
fingerprint, stale plan) are fixed by the conformance suite — your implementation must pass it.

## Development rules

- **Server-only repo** (see AGENTS.md): stdlib (`strings`, `strconv`, `net/url`, `crypto/tls`,
  `crypto/sha256`, `encoding/hex`, `sort`, `context`, `time`) and maps are allowed. Do **not**
  rewrite them with `webtyp.com/fmt`.
- RouterOS client: `github.com/go-routeros/routeros/v3` (v3.0.1). Only `mikrotik/routeros` imports it.
- **No string literals in logic**: every menu path (`"/ip/dhcp-server/lease"`), property
  (`"mac-address"`), list name and marker is a constant.
- Tests in `tests/` (package `tests`), runner `gotest ./...`. Never export a symbol only for tests.
- No `TODO`, no commented-out code, no stubs. Delete the gonew stub (`mikrotik.go`'s `type Mikrotik
  struct{}` / `New()`).

## Design gate

**1. Prior art.**
- **Terraform provider `terraform-routeros/routeros`**: manages RouterOS objects declaratively over the
  API/REST, one resource per object. We adopt "declarative desired state → diff → apply" but keep a
  much narrower surface (only what `webtyp.com/network` describes).
- **Ansible `community.routeros`** (`api_modify`, `api_find_and_modify`): idempotent edits keyed on
  properties, with `--check` mode. Adopted: idempotency and "find our objects by a key" (the
  `[velty]` comment marker).
- **`go-routeros/routeros`**: the de-facto Go API client (MIT). Used as the transport instead of
  re-implementing the API sentence protocol.
- Why different: neither tool knows about "hosts and access levels"; they manage raw objects. This
  package is the translation layer from a vendor-neutral contract to RouterOS objects.

**2. Novice-name test.** `mikrotik.Open(url)` (like `sql.Open`), `mikrotik.EnvRouterURL`,
`gw.Version()`, packages `v6`/`v7` (the versions an operator knows), `routeros.Commander` /
`Run(sentence…)` (the API's own words: "sentence", "command").

**3. Complexity ledger.**
```
Concepts the developer must learn   +1 (Open + ROUTER_URL); dialects are invisible to the caller
Files they must touch to do X       registering a device: 0 router-side (was: 2 objects by hand)
Lines at the call site              gw, err := mikrotik.Open(os.Getenv(mikrotik.EnvRouterURL)) — 1
Ways to do the same thing           0 (no other RouterOS integration exists in the ecosystem)
```

**4. Where it belongs.** A vendor implementation of a framework port lives in its own repo (like
`postgres` for `storage`). Server-only, so it is not a domain module.

**5. What it deletes.** The gonew stub only. On the router, the hand-made per-MAC rules are not
deleted by this code (it never touches unmanaged objects); they become dead below the `[velty]` drop
and are removed by the operator.

## Stage 1 — transport: `routeros/` (package `routeros`)

`routeros/commander.go`:

```go
package routeros

// Record is one reply sentence: property name -> value.
type Record map[string]string

// Reply is the full answer to one command.
type Reply struct {
	Records []Record // the !re sentences
	Done    Record   // the !done sentence (holds "ret" after an add)
}

// Commander runs one API command, e.g.
// Run("/ip/dhcp-server/lease/print") or
// Run("/ip/dhcp-server/lease/add", "=mac-address=…", "=address=…").
type Commander interface {
	Run(sentence ...string) (Reply, error)
}

// PropRet is the property of Done that carries the id returned by "add".
const PropRet = "ret"
```

`routeros/client.go`:

```go
// Client is a Commander over a live API session.
type Client struct{ /* *routeros.Client from go-routeros */ }

// Dial opens an API session. tls=true uses API-SSL without certificate
// verification (see docs/ARCHITECTURE.md "Connecting").
func Dial(address, user, password string, useTLS bool) (*Client, error)

func (c *Client) Run(sentence ...string) (Reply, error) // maps go-routeros Reply.Re[i].Map / Reply.Done.Map
func (c *Client) Close() error
```
Use `DialTimeout` / `DialTLSTimeout` with a `const dialTimeout = 10 * time.Second`.
A `*routeros.DeviceError` (a `!trap`) is returned wrapped with the command path:
`fmt.Errorf("routeros %s: %w", sentence[0], err)`.

## Stage 2 — dialect `v6/` (package `v6`)

`v6/gateway.go`:

```go
package v6

// Gateway implements network.Gateway for RouterOS 6.x.
type Gateway struct{ C routeros.Commander }

func New(c routeros.Commander) *Gateway

var _ network.Gateway = (*Gateway)(nil)
```

Constants (`v6/names.go`) — the only place these literals exist:

```go
const (
	ManagedMarker = "[velty]" // comment prefix of every object this package owns

	ListLocal            = "velty-local"
	ListInternetFiltered = "velty-internet-filtered"
	ListInternet         = "velty-internet"

	WANList = "WAN" // interface list that holds the Internet uplink(s); must exist on the router

	PathResource   = "/system/resource"
	PathDHCPServer = "/ip/dhcp-server"
	PathLease      = "/ip/dhcp-server/lease"
	PathFilter     = "/ip/firewall/filter"
	PathNAT        = "/ip/firewall/nat"
	PathARP        = "/ip/arp"
	PathBridge     = "/interface/bridge"
	PathIfaceList  = "/interface/list"

	StaticOnlyPool = "static-only"
)
```
plus the property names you use (`PropID = ".id"`, `PropComment = "comment"`, `PropMAC = "mac-address"`,
`PropAddress = "address"`, `PropServer = "server"`, `PropAddressLists = "address-lists"`,
`PropDynamic = "dynamic"`, `PropStatus = "status"`, `PropHostName = "host-name"`, …) and the command
verbs (`"/print"`, `"/add"`, `"/set"`, `"/remove"`).

Comment for managed objects: `ManagedMarker + " " + name`. Baseline objects use fixed names:
`"[velty] internet"`, `"[velty] internet filtered"`, `"[velty] block"`, `"[velty] dns filter udp"`,
`"[velty] dns filter tcp"` (constants).

Access → list: `AccessLocal → ListLocal`, `AccessInternetFiltered → ListInternetFiltered`,
`AccessInternet → ListInternet` (one unexported function, single switch).

### Reading the router (`v6/read.go`)

One unexported `snapshot` struct filled by a single `read()` that prints, once each: DHCP servers,
leases, forward filter rules, dstnat rules, ARP, interface lists. Every Plan/Apply/Connections/
Discover starts from a fresh `read()`. Filter client-side (no API `?` queries).

- The DHCP server named `Settings.DHCPServer` must exist → else error
  `fmt.Errorf("mikrotik: DHCP server %q not found", name)`; its `interface` property is the LAN
  bridge.
- Interface list `WANList` must exist → else `mikrotik: interface list "WAN" not found`.
- Managed = comment starts with `ManagedMarker`. Unmanaged lease = static (`dynamic=false`) without
  marker. Unmanaged Internet rule = forward filter, `action=accept`, non-empty `src-mac-address`,
  not disabled, without marker.

### `Plan` (`v6/plan.go`)

Produce the same change semantics as `webtyp.com/network/mem` (the conformance suite checks them):

1. `d.Validate()`.
2. Settings → changes on the DHCP server (`address-pool`, `add-arp`) and the bridge (`arp`) per the
   ARCHITECTURE table; a field already at the desired value produces no change. `Object`:
   `"dhcp-server <name>"`, `"bridge <name>"`.
3. Baseline: each of the 3 forward rules and 2 dstnat rules missing or different (dstnat
   `to-addresses` ≠ `FilterDNS`) → `ChangeAdd` / `ChangeUpdate`, `Object` = its comment.
4. Hosts, in the given order: managed lease with same MAC → update if `address`/`address-lists`/
   comment differ; unmanaged static lease with same MAC → `ChangeAdopt`; else `ChangeAdd`.
   `Object`: `"dhcp lease <ip>"`. Conflict when an unmanaged static lease with a different MAC holds
   the IP (reason `fmt.Sprintf("IP %s is held by unmanaged lease of %s", ip, mac)`).
5. Managed leases whose MAC is not desired → `ChangeRemove`.
6. Warnings: unmanaged Internet rules (enabled) whose MAC is not desired, reason exactly
   `"has Internet by a hand-made rule but is not registered"`.
7. Fingerprint: SHA-256 (hex) over the canonical text of (a) every record of the snapshot that Plan
   read, sorted by path then `.id`, with keys sorted, and (b) the change list in order. Equal router
   state + equal desired ⇒ equal fingerprint; any router edit ⇒ different.

Change ordering inside `Plan.Changes` = the apply order from ARCHITECTURE ("Apply order").

### `Apply` (`v6/apply.go`)

Re-plan; compare fingerprint → `network.ErrPlanStale`; conflicts → `network.ErrConflicts`; then run
the changes in order. Firewall/NAT adds go at the **top** of their chain: `=place-before=<.id of the
first rule in that chain>` (omit when the chain is empty). Adopt = `set` on the existing lease with the
managed comment and desired fields. If a command fails mid-way, stop and return the error (the next
Plan shows what is left — never retry silently).

### `Connections` and `Discover` (`v6/status.go`)

- `Connections`: leases with `status=bound` → `SourceDHCP` (`mac-address`, `active-address` or
  `address`, `host-name`); ARP entries (`complete=true`) whose MAC has no bound lease → `SourceARP`.
- `Discover`: unmanaged static leases and unmanaged Internet rules merged by MAC (`Name` from the
  lease comment, else the rule comment; `Internet` true when an enabled rule exists).

MACs are compared and returned upper-case (`strings.ToUpper`).

## Stage 3 — dialect `v7/` (package `v7`)

```go
package v7

// Gateway implements network.Gateway for RouterOS 7.x. Every command used
// by the network contract is identical in RouterOS 6 and 7, so it delegates
// to v6; a difference is added here as a method override, never as a
// version check inside v6.
type Gateway struct{ *v6.Gateway }

func New(c routeros.Commander) *Gateway { return &Gateway{v6.New(c)} }

var _ network.Gateway = (*Gateway)(nil)
```

## Stage 4 — root package (`mikrotik.go`, replaces the gonew stub)

```go
package mikrotik

const (
	EnvRouterURL = "ROUTER_URL"
	SchemeAPI    = "routeros"
	SchemeAPITLS = "routeros+tls"
	portAPI      = "8728"
	portAPITLS   = "8729"
)

var (
	ErrScheme             = errors.New("mikrotik: ROUTER_URL scheme must be routeros:// or routeros+tls://")
	ErrUnsupportedVersion = errors.New("mikrotik: unsupported RouterOS version")
)

// Gateway is a network.Gateway connected to one router.
type Gateway struct {
	network.Gateway
	version string
	conn    *routeros.Client
}

// Open parses rawURL (see EnvRouterURL), connects, reads the RouterOS
// version from /system/resource and selects the v6 or v7 dialect.
func Open(rawURL string) (*Gateway, error)

// Version is the RouterOS version string reported by the router, e.g. "7.20.2 (stable)".
func (g *Gateway) Version() string

func (g *Gateway) Close() error
```
Dialect selection: unexported `func dialectFor(version string, c routeros.Commander) (network.Gateway, error)`
— major `6` → `v6.New`, `7` → `v7.New`, else `fmt.Errorf("%w: %s", ErrUnsupportedVersion, version)`.
Missing user in the URL → error `mikrotik: ROUTER_URL has no user`.

## Stage 5 — tests (`tests/`)

1. `tests/fakeros_test.go` — an in-memory RouterOS emulator implementing `routeros.Commander`:
   tables per menu path; `/print` returns all records; `/add` assigns `.id` = `*1`, `*2`… and returns
   it in `Done["ret"]`; `/set` and `/remove` by `=.id=`; `place-before` inserts before that id.
   Seeded with: `/system/resource` (`version` configurable), DHCP server `dhcp1` on interface `br1`,
   bridge `br1` with `arp=enabled`, interface list `WAN`. Unknown path or verb → error (never a
   silent empty reply).
2. `tests/conformance_test.go` — `conformance.Run` over a fixture using the emulator, once with
   `v6.New` and once with `v7.New`. `Settings()` → `{DHCPServer: "dhcp1", DynamicPool: "pool1",
   FilterDNS: "1.1.1.3", Unregistered: UnregisteredNoAddress}`. `AddUnmanaged` writes a static lease
   (and, when `Internet`, an accept rule by `src-mac-address`) without the marker.
3. `tests/enforcement_test.go` — after applying `{h1 AccessInternetFiltered, h2 AccessInternet}` with
   `UnregisteredNoAddress`, the emulator holds: 2 leases with the right `address-lists` and comments,
   the 3 forward rules at the top in order (internet, internet filtered, block), the 2 dstnat rules
   with `to-addresses=1.1.1.3`, DHCP server `address-pool=static-only` `add-arp=yes`, bridge
   `arp=reply-only`. Then switching to `UnregisteredLocal` restores `address-pool=pool1`, `arp=enabled`.
4. `tests/unmanaged_test.go` — a pre-existing unmanaged forward rule and lease are byte-identical
   after any Apply (except an explicit adopt of that lease).
5. `tests/open_test.go` — `dialectFor` behaviour via `Open` is not reachable without a router, so test
   URL parsing through `Open` errors only: wrong scheme → `ErrScheme`; no user → the no-user error.
   (Do not export `dialectFor`.) Version selection is covered by the integration tests.
6. `tests/connections_test.go` — bound leases → `SourceDHCP`; ARP-only → `SourceARP`.

### Integration (build tag `routeros_integration`)

- `docker-compose.test.yml` at the repo root: services `ros6` (`evilfreelancer/docker-routeros:6.49.19`)
  and `ros7` (`evilfreelancer/docker-routeros:7.20.2`), `privileged: true`,
  `devices: [/dev/net/tun, /dev/kvm]`, cap `NET_ADMIN`, API port 8728 published as `18728` (ros6)
  and `28728` (ros7). Default CHR login: user `admin`, empty password.
- `tests/integration_test.go` (`//go:build routeros_integration`): for each of
  `routeros://admin:@127.0.0.1:18728` and `…:28728`: `Open`, assert `Version()` major matches, prepare
  the router (bridge `velty-test`, address `10.99.0.1/24`, pool `velty-test-pool`, DHCP server
  `velty-test`, interface list `WAN` if missing), then `conformance.Run` with a fixture that removes
  every `[velty]` object and every test unmanaged object between subtests.
- README section "Integration tests": `docker compose -f docker-compose.test.yml up -d`, wait ~60 s,
  `gotest -tags routeros_integration ./tests/...`.

## Stage 6 — docs

- `README.md`: purpose (first paragraph of ARCHITECTURE), `ROUTER_URL` format, a 6-line usage example
  (`Open`, `Plan`, `Apply`, `Close`), the two operator consequences from ARCHITECTURE ("Consequences
  the operator must know"), the router-side hardening (dedicated user `group` with `api,read,write`;
  `/ip service set api address=<server>/32`), and the integration-test section.
- Verify `docs/ARCHITECTURE.md` against the code; fix names that differ.

## Acceptance criteria

- `gotest ./...` green (emulator conformance for v6 and v7).
- `go vet ./...` clean; `grep -rn "go-routeros" --include=*.go . | grep -v "^./routeros/"` → empty.
- `grep -rn "type Mikrotik struct" .` → empty.
- `grep -rn '"\[velty\]' --include=*.go . | grep -v names.go | grep -v _test.go` → empty (marker only
  as a constant).

| Stage | Files | Done when |
|---|---|---|
| 1 | `routeros/commander.go`, `routeros/client.go` | compiles |
| 2 | `v6/*.go` | implements `network.Gateway` |
| 3 | `v7/gateway.go` | implements `network.Gateway` |
| 4 | `mikrotik.go` | `Open` parses, dials, selects |
| 5 | `tests/*.go`, `docker-compose.test.yml` | `gotest ./...` green |
| 6 | `README.md`, verify ARCHITECTURE | done |
