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
`sdd export && cat SPEC.md`.

## LOAD
1. `sdd export` then read SPEC.md. If no spec.db → tell the user to run sdd-spec. Stop.
2. Pick the target: `T<n>` (that task), `--next` (lowest `.`/`~`), or `--all`.
3. High blast radius (shared module, data, public interface)? Run sdd-review first.

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
sdd set-task <id> --status ~     # wip
# edit code per plan
# run the verification command
sdd set-task <id> --status x     # pass -> done
```
On failure: do NOT retry blindly — invoke **sdd-backprop** first.

## VERIFICATION
A task is `x` only if the oracle exits 0, every cited invariant has its named
passing test, and the full suite still passes. `sdd check` must pass (SPEC.md ==
spec.db). Commit after each task.
