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
- Defines what a realm and a project say about each other (change realm-leaf-naming): a project's `umbrella.yaml` pins
  its graph (`graph` and `graph_version`, both or neither, such as `software` at `0.1.0`), its runtime (`runtime` and
  `runtime_version`, both or neither, such as `niflheim-local` at `0.1.0`; `leaves.yaml` carries the same pin as a
  `runtime` block with `realm` and `realm_version`) and, for each leaf, the realm that provides it and the leaves it
  consumes. A top-level `realm` or `realm_version` is refused with the way to pin a graph. A build realm's `pack.yaml`
  offers a leaf recipe (template, implementer prompt, checks), declares the services its leaves `needs` (an older
  pack names `services` itself), and
  may have no graph; `leaves.yaml` is the spec stage's list of a product's
  leaves and their realms.
- Says what a build realm needs and what a runtime realm provides (change gallery-runtime): `pack.yaml`'s
  `needs` lists the services its leaves need (`gungnir.Needs`: `postgres`, a Postgres-compatible database, and
  `s3`, an S3 API; each named once), and `provides` maps each need a runtime realm meets to one of its
  `services` of the same name (`Pack.Provider(need)`). A pack with a `leaf` and `provides` or `product` is refused: a
  runtime realm provides no leaf (and, with a `sandbox`, no stack). A runtime realm's `product.start` is a regular file of the
  pack that Odin's init copies to the project root as `start.sh`.
- Gives a graph a schema version: `graph.yaml` states `schema: 2` (required, an integer; 2 is what
  `schema.GraphSchema` reads now), and a Gungnir refuses a graph that needs a higher one. It refuses schema 1 too, which
  declares no node's input and output (change typed-boundaries). `factory.yaml` names the
  store of graph packs with `graphs:` (`dir`, a clean absolute path, as `realms:` is). A graph pack is a
  `pack.yaml` with a `graph`; `HoldsRecipe` (of `Pack`) says whether it also holds a recipe (`checks`, `leaf`,
  `leaves`, `sandbox` or `services`), and only such a pack serves the leaves that pin no realm.
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
- Defines the grader's contract (spec §11): the `grade` node, which is the last node of a graph and has no
  other field; the `grader:` block of `factory.yaml` (`tasks_dir`, `suite_dir`, `timeout`, `memory`,
  `cpus`: all five are set, none has a default, the two folders are separate and `memory: none` with
  `cpus: 0` is the written choice of no limit); `score.json` (`ygg.score/v1`: the reward, the fail-to-pass
  and pass-to-pass tallies, the categories and their weights) and a suite's `manifest.json`
  (`ygg.suite/v1`). A score that does not add up and a manifest that can be read two ways are refused.
- Defines the ledger events of grading (`grade_started`, `grade_finished`, `grade_refused`), of routing
  (`routing_index_built`, `pointer_contract_checked`) and of dependency checks (`deps_checked`,
  `deps_bump`). A reward of 0 is still written. A refusal has one reason: `retired`, `missing_suite`,
  `brief_changed`, `bad_manifest` or `bad_task`.
- Defines `Usage.BilledUSD`, the part of a session's cost that a paid API bills. The dollar caps count it, and not the
  cost figure of a subscription's sessions, which is what they would cost on the API and which nobody charges.
- Defines `Tracker`, and the `Board`, `Card` and `Column` it returns: six columns in reading order, a board that is
  checked (no unknown column, no task twice, a blocked card says why).
- Defines the ledger fields that `ygg metrics` reads: the event `dispute_ignored`, `ygg.finding.origin` (`check` or
  `review`) on every finding line, the bench numbers of a measured routing index (`ygg.routing.bench`,
  `ygg.routing.baseline`, `ygg.routing.threshold`, each from 0 to 1 and written when 0) and the build of ygg that wrote a
  run (`ygg.factory.version`, on `run_started` and `run_resumed`).
- Defines the `routing` block of a graph pack's `pack.yaml` (`threshold`, and `bench`, the bench file from
  the project root): the share of bench questions an index must answer right first before it ships. The
  hard approval point `pointer-contract` is in the schema of the gates.
- Validates configs against those schemas.
- Takes every interface change first: one PR here, plus one in each affected component (spec §1).

## Contracts

- A contract (`schema.Contract`) is a versioned data boundary of the factory (change typed-boundaries): an id such as
  `ygg.plan/v1`, a JSON Schema in `schema/` and a Go type. `Contracts()` lists them, `SchemaFile` names the embedded
  schema and `SchemaFS` hands the schemas to the work image.
- A schema uses only the shared keywords, sets `additionalProperties: false` on every object that has `properties`
  and a schema on every map, and leaves prose fields free text, bounded for safety only. Every agent output may carry
  `blocked` (`gungnir.Blocked`, a reason) and `notes`.
- `Validate` returns a `*ValidationError`: JSON pointers with a sentence each, sorted, at most 50, so the list can go
  back to the model. `Decode` reads one valid document and refuses unknown fields and a second document.
  `DecodeAnswer` reads a session's whole answer: one document, bare or in exactly one code fence, and nothing else.
- Every agent node, every loop (its implementer) and every reviewer of a graph declares `output`, one contract of
  `Contracts()`, and may declare `input`: node ids (that node's output) or the built-in sources (`brief`,
  `decisions`, `task`, `change`, `open_findings`, `check_results`, `rejection`, `refusal`, `record`, `date`;
  `BuiltinSources()`). `inputs:` folders are gone. `ParseGraph` calls `Graph.CheckTypes`, which refuses a graph whose
  joins do not type-check and names the node and the field: an input with no source before it, a `reviews:` node
  or a `publish` source with no output, a `tasks` node whose `from` is not the plan (`ygg.plan/v1`), a gate's
  `deviation_from` that is not a conformance (`ygg.conformance/v1`), a node id equal to a built-in source, a loop whose output is not `ygg.implement/v1` or a reviewer's not
  `ygg.review/v1`, `task` outside a foreach body, `change`, `open_findings` or `check_results` for a node that is not a loop or its reviewer, or `record`
  before the grade, and a node a gate re-runs without `rejection` in its input or a `back_to` target without
  `refusal` (`rejection` is a person's gate rejection; `refusal` the factory's: readiness, merge, pointer or the
  owner-decision check). `Graph.CheckPrompts(dir)` is the load rule for prompts (ruling R66): a prompt names an
  input when it holds, in backticks, a built-in source or the id of a node of the graph that declares an output
  (`PromptInputs` reads that), and it may name only the inputs its session declares. It walks every agent node,
  loop, reviewer and foreach body, skips a session with no `prompt` (a realm's loop), and refuses a missing
  prompt file or a path outside the pack, naming the graph file, the node (and reviewer), the prompt and the input.
  Prose without backticks is not checked; a backticked name inside a fenced code block counts as named, so an
  example node id or a node's own id is written as plain prose. `ParseGraph` does not call it, since it has no pack folder; the
  fixture `testdata/graphs/software.yaml` has no prompt files beside it, so it is not run against it. After
  the `grade` node only a read-only or scratch agent that outputs `ygg.retro/v1` may follow.
- `Task.Output` names the contract a session returns, and `Result.Submitted` holds the output document of the last
  accepted `submit`; when that call named a draft file, the adapter reads the file from the workspace. An adapter that hands outputs back through `ygg-submit` implements `Submitter`.

## Depends on

- Nothing.

## Source

The v0 engineering spec, §1, §2.1, §2.2. When this page and the spec differ, the spec wins.
