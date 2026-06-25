# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

`sdd` is a SQLite-backed spec engine. The spec lives in a database; `SPEC.md` is a
generated read-only view. The full user-facing rationale and vocabulary is in
`README.fr.md` (French) — read it for the *why*. This file covers the *how* for code work.

## Commands

```bash
go build -o sdd .              # build the CLI
go test ./...                  # run all tests (one package, ~100 tests)
go test -run TestMigrateChainV2toV4   # single test by name
go vet ./...                   # lint
go tool sqlc generate          # regenerate the db/ query package from db/schema + db/query.sql
```

Tests use the real cobra root in temp dirs against a temp XDG store — no mocks. There is
no CI config; the full gate is `go test ./...` + `go vet ./...` **plus**
`go tool sqlc generate && git diff --exit-code` — generated code that is stale or
uncommitted fails the gate (V54), since `go test`/`go vet` alone do not see a stale codegen.

## sqlc data layer (F8)

`sqlc` is pinned as a tool dependency (the `tool` directive in `go.mod`); run it via
`go tool sqlc`, never a global install, so the version is reproducible. It generates the
typed query package under `db/` from `db/query.sql` against the DDL in `db/schema/`.
**sqlc is build-time only — it never runs migrations** (V52); the runtime schema is still
driven by `userVersion` + the `migrations` map in `schema.go`. The DDL in `db/schema/*.sql`
is the *single source* the runtime migrator embeds via `go:embed` AND sqlc reads for
codegen, so the codegen schema and the runtime schema cannot diverge (V51). All SQL lives
in `db/query.sql` as named queries; no hand-written SQL string literal survives in command
or render code (V50).

## Architecture

**spec.db is the single source of truth; SPEC.md is a generated artifact.** Never hand-edit
SPEC.md — `sdd check` fails on any byte drift (V6). Every mutation re-exports SPEC.md
atomically (write-temp + rename, V8), so file and db never diverge.

**One global db, many projects.** There is no per-repo db. The store is a single SQLite
file at `$XDG_CONFIG_HOME/sdd/spec.db` (else `~/.config/sdd/spec.db`), opened by
`openGlobalDB` (`store.go`). Every durable row + feature carries a `project_id`
(`schema.go`); a project is identified by canonical git origin URL, falling back to main
worktree path (`resolve.go`: `canonURL`, `lookupProject`). Clones/worktrees of one repo
share a spec. **Every query filters by `project_id`** — cross-project leakage must stay
structurally impossible (V20, V26).

**Durable vs ephemeral cut.** Durable tables (invariant, interface, bug, research, test)
survive across features. Ephemeral tables (feature, goal, constraint, task, unknown, gate)
are wiped via `ON DELETE CASCADE` from `feature` (V4). Cites are typed join tables with
real FKs (`task_cites_inv`, `task_cites_iface`, `bug_fix`) — a task cannot cite a
non-existent invariant (V5). FKs are enabled per-connection via DSN pragma in `db.go`.

**Ordinals vs PKs.** Rows render and are cited by a per-project ordinal (`V<n>`, `T<n>`,
`B`, `R`, `U`, `F`), NOT the global PK. `nextOrd`/`nextTaskOrd` (`resolve.go`) compute it;
the `fmt*Line` helpers in `export.go` are the *single source* of line rendering, shared by
`renderSpec` (whole file) and `sdd show` (one row) so a show line is byte-identical to its
SPEC.md line (V18).

### Key flow patterns

- **Mutation command**: wrap logic in `runMutation` (`mutations.go`) — it opens the global
  db, resolves the project, runs your `fn(db, pid)`, then re-exports. Do the write inside a
  transaction (`queryer` interface in `resolve.go` works for both `*sql.DB` and `*sql.Tx`).
- **Read command**: must be *pure* — no re-export, no mutation. `read_purity_test.go`
  drives every read through the root and asserts SPEC.md is byte-unchanged AND check still
  passes. Add new read commands to that oracle.
- **New command**: register in `newRootCmd` (`main.go`).

### Schema migrations

`schema.go` is the migration anchor. To add a version: bump `userVersion`, add one entry to
the `migrations` map keyed by the new version with its additive DDL constant — nothing else.
`applySchema` (fresh db) and `migrate` (existing db) both derive from the same constants, so
**fresh == migrated by construction** (V45) — `TestFreshEqualsMigratedSchemaV<n>` enforces
this. `migrate` loops `uv+1..userVersion` stamping each *literal* version (never the moving
`userVersion` constant, which would skip steps — this was review BLOCK-1). Migrations are
additive only (ALTER/CREATE), never destructive.

## Conventions

- **Invariants drive the design.** Code comments cite invariants as `V<n>` and reference the
  rule they enforce. When changing behavior, check whether an invariant covers it; if a bug
  reveals a missing one, that's the `sdd-backprop` move (bug → new invariant).
- **The `sdd-*` skills orchestrate the spec lifecycle** (grill → spec → research → review →
  build → backprop → deepen). All durable spec writes go through the `sdd` CLI, never manual
  SPEC.md edits.
- SPEC.md and spec.db are gitignored; `README.fr.md` is versioned. The built `sdd` binary is
  gitignored too.
