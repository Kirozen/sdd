---
name: sdd-spec
description: |
  Sole mutator of the spec. Translates intent into sdd CLI calls: invariants,
  interfaces, tasks, and feature goals/constraints. Never hand-edits SPEC.md —
  that file is the generated read-only view of spec.db. Triggers on "write the
  spec", "add an invariant", "spec this feature", or /sdd-spec. Ingests handoffs
  from sdd-grill (goal/constraints), sdd-research (findings), sdd-review
  (invariants).
---

# sdd-spec — spec mutator (via sdd)

The other skills produce material; sdd-spec persists it through the CLI. The db
is the source of truth; SPEC.md is generated (`sdd export`). Read it for
context, never edit it (V3, V6).

## DISPATCH
- New feature from an idea → run sdd-grill first if fuzzy, then land it here.
- Add durable truth (invariant/interface/bug) → the add-* commands.
- Amend an existing row → `sdd edit <kind> <id> --text "<new>"` (id stays stable, V12).

## DURABLE vs FEATURE
Durable (persists across features) — write freely, they survive wipe:
```
sdd add-invariant "<testable invariant>"          # -> V<n>
sdd add-interface <kind> <name> "<signature>"     # cite key I.<name>
sdd add-bug "<cause>" --fix V<n>                  # backprop log
```
Feature-scoped (wiped per feature):
```
sdd add-goal "<line>" --feature <id>
sdd add-constraint "<bullet>" --feature <id>
sdd add-task "<task>" --feature <id> --cites V2,I.init
sdd add-cite <T-ord> V3,I.foo                     # cite an EXISTING task
```
Landing a whole spec at once? Batch every write in one transaction with
`sdd apply` — TAB-delimited `add-*` lines on stdin, all-or-nothing, a single
final re-export; a leading `new-feature` sets the current feature.

## RULES
- `--cites` must reference existing V<n>/I.<name>; the FK rejects orphans (V5).
- Every mutation re-exports SPEC.md atomically; run `sdd check` if unsure of drift.
- Show the user what you will run, then run it. The CLI is the diff.
- `sdd --help` lists every command; `sdd <cmd> --help` for one.

## HANDOFF
After tasks exist, point the user at **sdd-review** (high blast radius) or
**sdd-build**.
