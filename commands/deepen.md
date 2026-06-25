---
description: Optional design-improvement pass for spare budget. Finds the shallowest module the spec touches and proposes refactors that shrink interfaces and hide decisions — behavior held constant.
argument-hint: [module | "this feels shallow"]
---

Invoke the **sdd-deepen** skill (`skills/sdd-deepen/SKILL.md`). Treat `$ARGUMENTS`
as the module or smell to address. Research a deeper design and propose refactors
that shrink interfaces and hide decisions — tests green before and after, behavior
held constant. Propose edits via the `sdd` CLI, never silent rewrites.
