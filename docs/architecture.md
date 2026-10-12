---
title: Gungnir architecture
status: draft
date: 2026-10-12
related: [https://github.com/ouroboric-dev/yggdrasil/blob/main/docs/2-engineering/specs/2026-10-11-yggdrasil-v0-spec.md]
style: reference
---

# Gungnir architecture

The contract: Go interface types and JSON schemas for ledger events, `graph.yaml`, `pack.yaml` and `factory.yaml`.

## Responsibilities

- Defines every interface the components share: `Agent`, `Sandbox`, `StateStore`, `Remote` and
  `Tracker` (spec §2.2).
- Defines the schemas: `factory.yaml`, `graph.yaml`, `pack.yaml`, ledger events (with OpenTelemetry
  `gen_ai.*` field names) and findings.
- Validates configs against those schemas.
- Takes every interface change first: one PR here, plus one in each affected component (spec §1).

## Depends on

- Nothing.

## Source

The v0 engineering spec, §1, §2.1, §2.2. When this page and the spec differ, the spec wins.
