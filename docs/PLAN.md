---
PLAN: "feat: lazy, reconnecting RouterOS session; Open no longer dials"
TAG: v0.2.0
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 13927726603179970495
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
>
> Follow-up of the network administration master plan (private repo `veltylabs/mjosefa-cms`; you do
> not need it). The application that consumes this package must start even when the router is down
> (after a power cut both reboot, and the router may come up later), and must survive the router
> rebooting while it runs.

# Plan — `github.com/veltylabs/mikrotik` v0.2.0: lazy, reconnecting session

**The spec is [docs/ARCHITECTURE.md → "Connection lifecycle"](ARCHITECTURE.md)** — read it first.
Read also [AGENTS.md](../AGENTS.md): this repo is **server-only**; stdlib (`errors`, `sync`,
`strings`, …) is legitimate here — do NOT replace it with `webtyp.com/fmt`.

## Development rules

- No string literals in logic; error messages exactly as below.
- Tests in `tests/` (package `tests`), runner `gotest ./...`. Never export a symbol only for tests —
  every exported symbol below is used by `Open` itself.
- Never add a `replace`; no `TODO`, no commented-out code.

## Design gate

**1. Prior art.** `database/sql.Open` does not connect (connections are made lazily and re-made by
the pool after a failure); `redis/go-redis` and `jackc/pgx` pools reconnect transparently but never
replay a non-idempotent command; Kubernetes client-go re-establishes watches after a broken stream.
Adopted: `Open` validates only, dial on first use, redial on the next call, never replay.

**2. Novice-name test.** `routeros.NewSession(dial)`, `session.Run(...)`, `routeros.DeviceError`
(the router answered with an error), `mikrotik.New(session)`, `gw.Version() (string, error)`.

**3. Complexity ledger.**
```
Concepts the developer must learn   +1 (Session); dialects/transport unchanged
Files they must touch to do X       0 in the app (Open keeps its signature)
Lines at the call site              unchanged; Version() now also returns an error
Ways to do the same thing           0 — the eager dial in Open is DELETED
```

**4. Where it belongs.** Connection management is this package's transport concern
(`routeros/`); dialect selection stays in the root.

**5. What it deletes.** The eager dial + version read inside `Open`; the `conn *routeros.Client`
field and the embedded `network.Gateway` of `Gateway` (replaced by explicit methods).

## Stage 1 — `routeros/`

1. `routeros/client.go`: `Client.Run` returns router traps as the new exported type
   ```go
   // DeviceError is a RouterOS !trap: the router received the command and answered with an
   // error. The connection is fine.
   type DeviceError struct{ Path string; Err error }
   func (e DeviceError) Error() string { return "routeros " + e.Path + ": " + e.Err.Error() }
   func (e DeviceError) Unwrap() error { return e.Err }
   ```
   (replacing the `fmt.Errorf("routeros %s: %w", …)` wrap). Every other error is returned as is.
2. `routeros/session.go` (new):
   ```go
   // Conn is one live API connection.
   type Conn interface {
   	Commander
   	Close() error
   }

   // Session is a Commander that dials on first use and dials again after the
   // connection breaks. A command that fails because the connection broke returns
   // that error and is NEVER retried; the next command dials again. A DeviceError
   // keeps the connection. Safe for concurrent use: commands are serialized.
   type Session struct { /* mu sync.Mutex; dial func() (Conn, error); conn Conn; generation uint64 */ }

   func NewSession(dial func() (Conn, error)) *Session
   func (s *Session) Run(sentence ...string) (Reply, error)
   // Generation counts successful dials; it changes when a new connection was made.
   func (s *Session) Generation() uint64
   func (s *Session) Close() error
   ```
   `Run`: lock; if `conn == nil` → `dial()` (error → return it; success → `generation++`); run; on an
   error that is not a `DeviceError` (`errors.As`) → `conn.Close()`, `conn = nil`, return the error.

## Stage 2 — root package (`mikrotik.go`)

```go
// Gateway is a network.Gateway over one router, reached through a reconnecting
// Session. It detects the RouterOS version on every new connection.
type Gateway struct { /* session *routeros.Session; mu sync.Mutex; dialect network.Gateway; version string; generation uint64 */ }

// New builds a Gateway over session (Open uses it).
func New(session *routeros.Session) *Gateway

// Open validates rawURL (see EnvRouterURL) and returns a Gateway WITHOUT dialing.
func Open(rawURL string) (*Gateway, error)

func (g *Gateway) Plan(d network.Desired) (network.Plan, error)
func (g *Gateway) Apply(d network.Desired, expected network.Fingerprint) (network.Plan, error)
func (g *Gateway) Connections() ([]network.Connection, error)
func (g *Gateway) Discover() ([]network.Discovered, error)
// Version reads the version of the current connection (dialing if needed).
func (g *Gateway) Version() (string, error)
func (g *Gateway) Close() error

var _ network.Gateway = (*Gateway)(nil)
```

- Each of the four network methods first calls an unexported `current() (network.Gateway, error)`:
  if no dialect is cached **or** `session.Generation()` differs from the cached generation, run
  `/system/resource/print` through the session (this dials if needed), read `version`, select the
  dialect with the existing `dialectFor(version, session)`, cache dialect, version and generation.
  Then delegate.
  - Order matters: read the generation **after** the resource print (the print is what dials).
- `Open`: keep the URL validation exactly as today (`ErrScheme`, the no-user error, default ports);
  build the dial func (`routeros.Dial(host, user, password, useTLS)`) and return
  `New(routeros.NewSession(dial))`. **No network I/O in `Open`.**
- Delete the eager dial/version read and the old struct fields.

## Stage 3 — tests (`tests/`)

Add `tests/session_test.go` with a fake `Conn` built on the existing emulator (`fakeros_test.go`),
plus a "breakable" switch that makes the next `Run` return a plain transport error
(`errors.New("connection reset")`):

1. `Open("routeros://admin:x@127.0.0.1:1")` returns no error and does not dial (port 1 is closed;
   the call must return immediately). Its `Plan(...)` then returns a non-nil error.
2. Session dials once for two commands; after a transport error, the failing command is NOT
   retried (the emulator saw it zero extra times) and the NEXT command dials again
   (`Generation()` 1 → 2).
3. A `DeviceError` returned by the fake keeps the connection (`Generation()` unchanged after the
   next command).
4. Version switch: the dial func returns an emulator with `version=6.49.19 (stable)` first and one
   with `7.20.2 (stable)` after a break; `Version()` reports 6 then 7, and the conformance-relevant
   call (`Plan`) succeeds on both.
5. Concurrency: 20 goroutines calling `Connections()` on one Gateway → no race (`gotest` runs `-race`).

Update `tests/open_test.go` to the new behaviour (no dial in `Open`). The conformance tests keep
using `v6.New` / `v7.New` directly and must stay green. `tests/integration_test.go`: replace any use
of the old `Version()` with the `(string, error)` form.

## Stage 4 — docs

README: `Open` no longer dials; reconnect behaviour in 3 lines; `Version()` signature.
Verify ARCHITECTURE "Connection lifecycle" against the code, then **remove its STATUS note**.

## Acceptance criteria

- `gotest ./...` green (race detector on).
- `grep -n "routeros.Dial(" mikrotik.go` shows it only inside the dial func passed to `NewSession`.
- `grep -n "STATUS (remove" docs/ARCHITECTURE.md` → empty.

| Stage | Files | Done when |
|---|---|---|
| 1 | `routeros/client.go`, `routeros/session.go` | DeviceError, Session |
| 2 | `mikrotik.go` | Open without I/O; per-connection version detection |
| 3 | `tests/session_test.go`, `tests/open_test.go`, `tests/integration_test.go` | 5 cases green |
| 4 | `README.md`, `docs/ARCHITECTURE.md` | STATUS removed |
