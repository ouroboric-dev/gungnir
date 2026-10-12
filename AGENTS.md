# AGENTS.md: gungnir

The contract: Go interface types and JSON schemas for ledger events, `graph.yaml`, `pack.yaml` and `factory.yaml`.

Follow the Yggdrasil invariants:
https://github.com/ouroboric-dev/yggdrasil/blob/main/AGENTS.md#invariants
This file only adds rules for this repo.

## This repo

- Build and test: `go test ./...`. Lint: `go vet ./...`.
- Interfaces live in `gungnir`, never here (spec §1).
- Architecture: `docs/architecture.md`.
