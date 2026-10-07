# AGENTS.md — veltylabs/mikrotik

Working notes for AI agents. Design: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Mission

A concrete `webtyp.com/network` gateway for MikroTik RouterOS 6 and 7. It implements the contract;
it never redefines it. If the contract lacks something, the fix is a plan against
`webtyp.com/network`, never a local type.

## This repo is server-only — read before "fixing" imports

- It opens TCP connections and is **never** compiled to WASM/TinyGo. The ecosystem rules "no stdlib",
  "no `map`" protect WASM binary size and **do not apply here**: `strings`, `strconv`, `net/url`,
  `crypto/tls`, `crypto/sha256`, `context` and maps are legitimate. Do NOT replace them with
  `webtyp.com/fmt`.
- It must never be imported by a domain module (`veltylabs/modules/*`) — only by an application's
  composition root.

## Rules

- **Only touch what we created**: every object written carries `ManagedMarker` (`[velty]`) at the
  start of its comment. Reading unmanaged objects is fine; writing them is only allowed for an
  explicit `ChangeAdopt`.
- **One transport, two dialects** (`routeros/`, `v6/`, `v7/`). A version difference is an override in
  `v7/`, never an `if version` inside `v6/`.
- **No string literals in logic**: menu paths, property names, list names and markers are constants
  in the package that uses them.
- Tests in `tests/`, runner `gotest`. Integration tests only behind the `routeros_integration` build
  tag; never against a production router.
