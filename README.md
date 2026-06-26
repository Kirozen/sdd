# sdd — spec-driven development

*[Version française](README.fr.md).*

`sdd` is a SQLite-backed specification engine. The spec lives in a database;
`SPEC.md` is only a **generated Markdown view** of it (never hand-edited). Every
mutation re-exports `SPEC.md` atomically, so the file and the database never
diverge (`sdd check` guarantees it).

## Why

Writing code before settling the *what* and the *why* is expensive: a bad
assumption caught at spec time costs one question; caught after the build, it
costs a bug. `sdd` makes the spec **queryable, versionable and tamper-proof**:

- a clean cut between the **durable** (invariants, interfaces, bugs, research —
  survives across features) and the **ephemeral** (a feature's goals,
  constraints, tasks — erased with it);
- **real-key citations** (`V<n>`, `I.<name>`) protected by foreign keys: a task
  cannot cite an invariant that does not exist;
- a **global** database shared by every repository, each project kept isolated.

## The pieces of a spec

An `sdd` spec is made of a few typed **building blocks**. Each has a precise
reason to exist; it is this vocabulary that makes the spec queryable. If you only
remember one thing, make it the cut below.

### The fundamental cut: durable vs ephemeral

- The **durable** layer is the project's long-term memory: what stays true from
  one feature to the next. It is never erased in routine work.
- The **ephemeral** layer belongs to a **feature** — a unit of work. When the
  feature is finished or abandoned, all of its ephemeral rows are wiped; the
  durable layer survives. This is what keeps the spec from growing forever.

### Durable blocks

| Block | Key | What it is |
|---|---|---|
| **Invariant** | `V<n>` | A rule that must *always* hold, written to be testable. E.g. "a task cannot cite a non-existent invariant". The memory of the decisions made. |
| **Interface** | `I.<name>` | A stable contact point with the outside world (a command, an API, a file) and its signature. E.g. `I.export` = `sdd export → regenerates SPEC.md`. |
| **Bug** | `B<n>` | The trace of a past mistake, linked to the invariant that prevents its return. The lesson learned, carved in for good. |
| **Research** | `R<n>` | A verified external fact (official docs, RFC…) with its source. To ground a decision on a fact, not a guess. |
| **Test** | — | The *declared* link between an invariant and the test that proves it. Makes the "this code can no longer regress" promise auditable (`sdd cover`). |

### Ephemeral blocks (owned by a feature)

| Block | Key | What it is |
|---|---|---|
| **Feature** | `F<n>` | A unit of work: a goal and everything needed to reach it. |
| **Goal** | `§G` | The feature's objective in one sentence: what the code must do. |
| **Constraint** | `§C` | A non-negotiable limit or requirement: out-of-scope, mandated tech, a contract to honor. |
| **Task** | `T<n>` | A concrete implementation step, with a status and the durable blocks it cites. |
| **Unknown** | `U<n>` | A still-open question, *parked* rather than guessed. Moves from `open` to `resolved`. |
| **Gate** | — | The feature's review verdict (`go` / `no-go`): did it pass the adversarial review before the build? |

### Citations — the glue

A task declares what it depends on:
`sdd add-task "…" --cites V2,I.export`. Foreign keys forbid citing an invariant
or interface that does not exist — you cannot reference the void. This is what
makes the spec **tamper-proof**: `sdd refs V2` shows, in reverse, everything that
depends on `V2`, so nothing breaks silently when you touch it.

### The `V<n>` / `T<n>` … keys

Every numbered block carries a project-local **ordinal**: the first invariant is
`V1`, the second `V2`, and so on (likewise `T`, `B`, `R`, `U`, `F`). It is this
short key that you read in `SPEC.md` and pass to commands (`sdd show V2`,
`sdd set-task 7 --status x`).

## Installing as a Claude Code plugin

`sdd` ships as a **Claude Code plugin**: the skills and the `/sdd:*`
slash-commands are packaged, and the `sdd` binary is **provisioned automatically**
at session start.

```
/plugin marketplace add Kirozen/sdd
/plugin install sdd@sdd-marketplace
```

On the first `SessionStart`, a hook detects your OS/arch and downloads the
matching `sdd` binary from the plugin version's
[GitHub release](https://github.com/Kirozen/sdd/releases), verifies its
**SHA256**, then installs it into the plugin's `bin/` (added to the Bash tool's
`PATH`) — so the skills call `sdd` with no configuration. Provisioning is
**idempotent** (it does not re-download if the binary is present) and **never
blocking**: on failure (network, unsupported platform) it prints manual install
instructions and lets the session continue.

**Supported platforms**: macOS and Linux (including WSL), on `amd64` and
`arm64`. Native Windows is not supported in v1 — on Windows, use WSL or install
the binary by hand (`go install github.com/kirozen/sdd/cmd/sdd@latest`).

> **The repo dogfoods itself** — this repository *is* the plugin (the skills live
> under `skills/`, no longer under `.claude/skills/`: single source). To work on
> `sdd` with its own skills, install the plugin locally: `/plugin marketplace add ./`
> then `/plugin install sdd@sdd-marketplace`.

## The workflow (skills)

The `sdd-*` skills orchestrate the lifecycle; every durable write goes through
the `sdd` CLI, never through a manual edit of `SPEC.md`.

```
grill → spec → research → review → build → backprop → deepen
```

- **sdd-grill** — sharpens a fuzzy idea into a goal + constraints. One question at
  a time; unknowns are parked (`add-unknown`), never guessed.
- **sdd-spec** — the sole mutator of the spec: invariants, interfaces, tasks.
- **sdd-research** — gathers external facts; every finding cites its source.
- **sdd-review** — adversarial review: tries to refute the spec before any code,
  ending on a go / no-go verdict recorded by `sdd gate` (which `sdd guide` later
  reads back).
- **sdd-build** — implements task by task; flips status via `set-task`, links each
  test to the invariant it proves via `sdd add-test` (auditable with `sdd cover`).
- **sdd-backprop** — bug → invariant: on a failure, decides whether a new invariant
  would prevent the recurrence.
- **sdd-deepen** — optional design-improvement pass (spare budget).

To see where you stand: `sdd guide` reports, per feature, the inferred stage and
the recommended next skill; `sdd next` gives the next actionable task with its
goal and resolved citations.

## Commands

**Lifecycle**: `init`, `export`, `check`, `backup`, `import`

**Ephemeral mutations (per feature)**: `new-feature`, `add-goal`,
`add-constraint`, `add-task`, `add-cite`, `set-task`, `wipe-feature`,
`add-unknown`, `resolve-unknown`, `gate`, `rm-task`, `rm-goal`, `rm-constraint`

**Durable mutations**: `add-invariant`, `add-interface`, `add-bug`,
`add-research`, `add-test`, `edit`, `deprecate-interface`, `retract-invariant`,
`retract-interface`

**Batch**: `sdd apply` reads TAB-delimited `add-*` subcommands from stdin (one
per line) and applies them all **in a single transaction** — all-or-nothing, a
single final re-export; a leading `new-feature` sets the current feature. This
is the agents' bulk-write lever (sdd-spec).

**Reads (pure, no re-export)**: `show`, `list` (with `--pretty`, and
`--status`/`--feature` for tasks), `refs`, `status`, `next`, `todo`, `guide`,
`cover`, `search`, `projects`, `stats`, `usage`

Task statuses: `.` todo · `~` in progress · `x` done.
Unknown statuses: `open` · `resolved` (never deleted).
Gate verdicts: `go` · `no-go` (one per feature, the latest replaces).

**Retraction** (`retract-invariant`, `retract-interface`, `rm-task`, `rm-goal`,
`rm-constraint`) *actually* deletes a row — unlike `deprecate-interface`, which
only marks it. A **cited** durable row cannot be removed (the command refuses,
listing its citers); remove what cites it first. `retract-invariant` warns when
it also carries away the tests that prove it. `rm-goal`/`rm-constraint` target
the *n*-th line of a feature (`sdd rm-goal <F-ord> <n>`, 1-based, in displayed
order).

`add-cite <T-ord> <cite>…` attaches `V<n>`/`I.<name>` citations to an *existing*
task without recreating it (FK-guarded, V5) — the counterpart to `add-task
--cites` when you cite after the fact.

A few commands to find your way around: `sdd guide` (where each feature stands,
and which skill to run next), `sdd next` (the next actionable task, with its goal
and resolved citations), `sdd todo` (every unfinished task as TSV — a stable
column contract for scripts and agents; `--pretty` for a grouped human view),
`sdd cover` (which invariants are guarded by a test and
which are not), `sdd search <term>` (full-text search over the *content* of the
current project's rows — where `refs` searches by citation key), `sdd projects`
(every project in the global store with its counts; the only command that looks
beyond the current project), `sdd stats` (per-type volume counts — invariants,
interfaces, bugs, research, tests, unknowns, features, tasks broken down by
status — for the current project; `sdd stats --all` aggregates the whole store
and adds the project count and the `spec.db` file size), `sdd usage` (how many
times each command was invoked, success/failure and last call, sorted by
frequency — for the current project; `sdd usage --all` aggregates the whole
store). Every invocation records itself; a recording failure never interrupts the
command.

## Where the data lives

The global database is unique, outside any repository:

```
$XDG_CONFIG_HOME/sdd/spec.db   (defaults to ~/.config/sdd/spec.db)
```

It is created on the first command and migrated automatically on a schema bump.
A project is identified by its git remote URL (failing that, the path of its main
worktree): clones and worktrees of the same repo share the same spec. Local
`SPEC.md` and `spec.db` are gitignored; both READMEs are versioned.
