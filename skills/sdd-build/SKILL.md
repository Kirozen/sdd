---
name: sdd-build
description: |
  Plan-then-execute implementation against the spec.db tasks. Reads the generated
  SPEC.md for context, flips task status via the sdd CLI, and auto-invokes
  sdd-backprop on test/build failure. Triggers on "build", "implement the next
  task", "run the build", `build T<n>`, or /sdd-build.
---

# sdd-build — implement the spec

Single-thread plan→execute. The task list lives in spec.db; read it via
`sdd cat` (durables + unfinished features; `sdd cat --feature N` for one).

## LOAD
1. `sdd cat` to read the spec. If no spec.db → tell the user to run sdd-spec. Stop.
2. Pick the target: `T<n> --feature <f>` (that task — ords are per-feature, V117),
   `--next` (lowest `.`/`~`), or `--all`.
3. High blast radius (shared module, data, public interface)? Run sdd-review first.

Orient with the pure read commands: `sdd next` (next actionable task + its goal
and resolved cites), `sdd todo` (every unfinished task as TSV — machine-readable
for picking work), `sdd status`/`sdd guide` (per-feature stage). `sdd --help`
lists every command.

## PLAN (native plan mode)
For the chosen task:
1. Cite every invariant (V<n>) it lists. The plan must respect all.
2. Cite every interface (I.<name>) it touches. Preserve the shape.
3. List files to create/edit.
4. Verification contract — name the EXACT test(s) that prove each cited
   invariant. Which test, not "add tests".
5. Name the verification command (the external oracle). Green = done.

## EXECUTE per task
```
sdd set-task <T-ord> --feature <f> --status ~   # wip (task ords are per-feature, V117)
# edit code per plan
# run the verification command
sdd set-task <T-ord> --feature <f> --status x   # pass -> done
```
On failure: do NOT retry blindly — invoke **sdd-backprop** first.

## VERIFICATION
A task is `x` only if the oracle exits 0, every cited invariant has its named
passing test (`sdd cover` flags any invariant with no proving test), and the
full suite still passes. `sdd check` must pass (SPEC.md == spec.db). Commit after
each task.
