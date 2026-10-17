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
- Defines what a realm and a project say about each other (change realm-leaf-naming): a project's
  `umbrella.yaml` pins its lead realm and, for each leaf, the realm that provides it and the leaves it
  consumes; a build realm's `pack.yaml` offers a leaf recipe (template, implementer prompt, checks) and
  sandbox services under the same names in every realm, and may have no graph; `leaves.yaml` is the
  spec stage's list of a product's leaves and their realms.
- Defines the graph's nodes beyond the loop: `foreach` (a task shape repeated for every ready task, in the
  leaf and the realm of the task), scratch agents, `publish`, `tasks` and `init`, and `back_to` for a
  refusal that goes back into an earlier node.
- Reads a plan's `tasks.md` exactly into `Feature`s, applies the Definition of Ready (`CheckReady`),
  and defines `features.json`, which Odin alone writes.
- Describes a sandbox (`SandboxSpec`): the leaf or scratch folder it holds, the stack layer of the
  leaf's realm, and the services it starts beside the work container (each with an optional command, for
  an image that does nothing without one).
- Says what a realm's stack is (change realm-content): an image pinned by digest, or a build context in
  the realm, whose files (and the stage built, its `target`) are hashed into the local tag of the image `ygg sandbox build` builds from it.
  A realm may name a second stack for the acceptance checks that run on a snapshot of the whole project.
- Defines the design-conformance review (spec §4.2): a scratch reviewer (`reviews`) reads another scratch
  agent's output, its verdict (`Conformance`) is read exactly, and the spec gate reads it (`deviation_from`).
- Validates configs against those schemas.
- Takes every interface change first: one PR here, plus one in each affected component (spec §1).

## Depends on

- Nothing.

## Source

The v0 engineering spec, §1, §2.1, §2.2. When this page and the spec differ, the spec wins.
