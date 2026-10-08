# `veltylabs/mikrotik` — architecture

## What this is

`mikrotik` makes a **MikroTik router** (RouterOS) behave as a `webtyp.com/network` gateway: an
application describes which devices may use the network and how much of it, and this package
translates that into DHCP leases, firewall rules and DNS redirection on the router, through the
RouterOS API.

You meet it in the composition root of an application that administers its own network
(`mjosefa-cms`): `gw, err := mikrotik.Open(os.Getenv(mikrotik.EnvRouterURL))`.

It is **server-only**: it opens TCP connections and never reaches a browser/WASM build.

## Connecting: `ROUTER_URL`

```
routeros://<user>:<password>@<host>[:8728]        RouterOS API, plain
routeros+tls://<user>:<password>@<host>[:8729]    RouterOS API-SSL
```

Same shape as `DATABASE_URL`. `routeros+tls` encrypts the session but does **not** verify the
router's certificate: RouterOS ships self-signed certificates and has no public name to verify
against. The router-side mitigation is to accept the API only from the application server
(`/ip service set api,api-ssl address=<server>/32`) with a dedicated user whose group grants only
`api,read,write`.

## Connection lifecycle: lazy, reconnecting, never retrying

> STATUS (remove this note when the "lazy reconnecting session" plan lands): this section is the spec of that plan.

The router is not needed for the application to start, and it reboots (power cuts, upgrades). So:

- **`Open` does not dial.** It validates `ROUTER_URL` (scheme, user) and returns. A malformed URL is
  an error at startup — a configuration mistake is seen when deploying. An unreachable router is
  not: the first operation reports it, and the next one tries again.
- **The session dials on first use and redials after a broken connection.** A transport failure
  closes the session; the **next** command dials again. A RouterOS `!trap` (the router answered with
  an error) does not drop the session.
- **A failed command is never retried.** If the connection broke right after the router executed an
  `add`, re-running it could duplicate an object. The operation returns the error (`Apply` stops, as
  it always does), and the next `Plan` shows what is left.
- **The version is detected on every new connection**, not once: upgrading the router from 6 to 7
  while the application runs switches the dialect after the reconnect.

## Version independence

RouterOS 6 and 7 speak **the same API protocol** (ports 8728/8729), so there is one transport
(`mikrotik/routeros`, built on `github.com/go-routeros/routeros/v3`). What differs between versions
is the **commands**: menus and properties (most visibly wireless/CAPsMAN: `/caps-man` versus
`/interface/wifi/capsman`). So the version split is by **dialect**:

```
mikrotik/            Open(url): dial, read /system/resource version, pick the dialect
mikrotik/routeros/   transport: Commander (Run a sentence, get records), Dial
mikrotik/v6/         dialect for RouterOS 6.x — implements network.Gateway
mikrotik/v7/         dialect for RouterOS 7.x — embeds v6, overrides only what differs
```

Upgrading the router from 6 to 7 changes nothing in the application: `Open` detects the new version
on the next connection. A future REST transport (RouterOS 7 only) would be another `Commander`, and
the dialects would not change.

**Rejected:** splitting by transport (API vs REST). It does not address what actually changes between
versions (the commands) and REST does not exist on RouterOS 6.

## How access is enforced on the router

Everything this package creates carries the comment prefix **`[velty]`** (`ManagedMarker`). Anything
without it is hand-made configuration: reported (`Discover`, warnings, conflicts), never modified
except by explicit adoption.

| Desired state | RouterOS objects |
|---|---|
| Host (MAC, IP, name) | Static DHCP lease on `Settings.DHCPServer`: `mac-address`, `address`, `comment="[velty] <name>"` |
| Host access level | The lease's `address-lists`: `velty-local`, `velty-internet-filtered` or `velty-internet`. RouterOS adds the IP to that list while the lease is bound |
| Internet allowed | Forward rules at the top of the chain: accept `src-address-list=velty-internet` → WAN; accept `velty-internet-filtered` → WAN; **drop** anything else from the LAN bridge → WAN |
| DNS filtering | `dstnat` rules (UDP and TCP 53) for `src-address-list=velty-internet-filtered` → `Settings.FilterDNS` |
| `UnregisteredNoAddress` | DHCP server `address-pool=static-only` + `add-arp=yes`; LAN bridge `arp=reply-only` (a hand-typed IP is ignored by the router) |
| `UnregisteredLocal` | DHCP server `address-pool=<Settings.DynamicPool>`; LAN bridge `arp=enabled` |

Why per-access **lists** and three fixed rules, instead of one rule per device (the hand-made setup
this replaces): registering, changing or removing a device touches **one lease**; the firewall never
changes again after the first apply.

Consequences the operator must know (repeated in the README):
- A registered device must take its IP from DHCP: the access lists are filled from bound leases.
- With the `[velty]` drop at the top, older hand-made per-MAC accept rules below it no longer grant
  anything. `Plan` warns about every such MAC that is not registered.

### Apply order

Adds/adoptions/updates of hosts → firewall and NAT baseline → settings (DHCP pool, ARP mode) →
removals. A device keeps working until its replacement exists; closing the network
(`UnregisteredNoAddress`) happens only after every registered host has its lease.

## Testing without a router

- `tests/` runs `webtyp.com/network/conformance` against both dialects over an **in-memory RouterOS
  emulator** (a fake `Commander` with tables per menu path) — no network, part of `gotest ./...`.
- Integration tests (build tag `routeros_integration`) run the same suite against RouterOS CHR in
  Docker (`evilfreelancer/docker-routeros`, tags `6.49.x` and `7.x`), started with
  `docker compose -f docker-compose.test.yml up -d`. Never against a production router.
