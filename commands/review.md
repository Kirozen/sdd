---
description: Adversarial senior review of the spec before any code is written. Refute, don't rubber-stamp. Ends on an explicit go / no-go gate.
argument-hint: ["review the spec" | feature N]
---

Invoke the **sdd-review** skill (`skills/sdd-review/SKILL.md`). Treat `$ARGUMENTS`
as the spec/feature under review (read it via `sdd cat`). Attack each axis for the
case where it breaks; every finding cites evidence (file:line or source).
Survivors harden into invariants via `sdd add-invariant`. End on GO or NO-GO.
