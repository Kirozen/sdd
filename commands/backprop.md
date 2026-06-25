---
description: Bug → spec protocol. Trace the root cause of a failure and decide whether a new invariant would catch recurrence, then persist it via the sdd CLI.
argument-hint: [bug | failing test]
---

Invoke the **sdd-backprop** skill (`skills/sdd-backprop/SKILL.md`). Treat
`$ARGUMENTS` as the bug or failing test. Trace the root cause, then persist via
`sdd add-bug --fix V<n>` and, when recurrence warrants it, `sdd add-invariant`.
This is the one move plain plan-then-execute skips.
