---
description: Sole mutator of the spec — invariants, interfaces, tasks, feature goals/constraints. Never hand-edits SPEC.md (it is generated).
argument-hint: [feature intent | "add an invariant"]
---

Invoke the **sdd-spec** skill (`skills/sdd-spec/SKILL.md`). Treat `$ARGUMENTS` as
the intent to translate into `sdd` CLI calls (add-invariant, add-interface,
add-task, add-goal, add-constraint, edit). The db is the source of truth; SPEC.md
is the generated read-only view — never edit it. Show the commands, then run them.
