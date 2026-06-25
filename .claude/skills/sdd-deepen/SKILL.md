---
name: sdd-deepen
description: |
  Optional design-improvement pass for spare budget. Finds the shallowest module
  the spec touches, researches a deeper design, and proposes refactors that shrink
  interfaces and hide decisions — behavior held constant, tests green before and
  after. Proposes edits via the sdd CLI, never silent rewrites. Triggers on
  "deepen this", "this module feels shallow", "use spare budget", or /sdd-deepen.
---

# sdd-deepen — pull complexity down

Optional. Run when you have spare budget to drain on design quality. Behavior is
held constant: the test suite must be green before and after.

## STEPS
1. Read the spec (`sdd cat`) and the code it touches. Find the
   shallowest module — wide interface, shallow implementation, leaked decisions.
2. Research a deeper design if needed (defer to sdd-research; log findings).
3. Propose refactors that shrink the interface and hide the decision. Express
   each as a concrete spec edit through the CLI, never a silent rewrite:
   ```
   sdd edit interface <id> --text "<smaller signature>"   # narrow the surface
   sdd add-invariant "<new invariant the deeper design guarantees>"
   sdd add-task "<refactor step>" --feature <id> --cites V<n>,I.<name>
   ```
4. Deprecate a superseded interface rather than deleting it:
   `sdd deprecate-interface <id>` (history kept, V11).

## RULE
Tests green before and after. Deepen proposes; the user approves; sdd-spec and
sdd-build carry it out. Shrinking the interface beats adding cleverness.
