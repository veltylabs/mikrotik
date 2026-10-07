# mikrotik
<img src="docs/img/badges.svg">

`mikrotik` makes a MikroTik router (RouterOS) behave as a `webtyp.com/network` gateway: an application describes which devices may use the network and how much of it, and this package translates that into DHCP leases, firewall rules and DNS redirection on the router, through the RouterOS API.

## ROUTER_URL format

```
routeros://<user>:<password>@<host>[:8728]        RouterOS API, plain
routeros+tls://<user>:<password>@<host>[:8729]    RouterOS API-SSL
```

## Usage Example

```go
gw, err := mikrotik.Open(os.Getenv(mikrotik.EnvRouterURL))
if err != nil {
	log.Fatal(err)
}
defer gw.Close()

plan, err := gw.Plan(desired)
if err == nil && !plan.Empty() {
	_, err = gw.Apply(desired, plan.Fingerprint)
}
```

## Consequences the operator must know

- A registered device must take its IP from DHCP: the access lists are filled from bound leases.
- With the `[velty]` drop at the top, older hand-made per-MAC accept rules below it no longer grant anything. `Plan` warns about every such MAC that is not registered.

## Router-side hardening

- The dedicated API user should have a `group` granting only `api,read,write`.
- Mitigate API exposure by setting the permitted address: `/ip service set api,api-ssl address=<server>/32`.

## Integration tests

Run the test suite against RouterOS CHR in Docker:

```bash
docker compose -f docker-compose.test.yml up -d
# Wait ~60 seconds for the routers to start
gotest -tags routeros_integration ./tests/...
```
