---
name: sdd-review
description: |
  Adversarial senior review of the spec before any code is written. Tries to
  refute the spec (read via sdd cat), citing evidence; survivors harden into
  invariants via the sdd CLI. Ends on an explicit go / no-go gate. Triggers on
  "review the spec", "red-team this", "is this plan sound", or /sdd-review.
---

# sdd-review — refute the spec before build

**Every finding cites evidence — file:line or a source. No evidence → flag `[unverified]`. Default to refuted.**

A refutation attempt, not "looks good". Read the spec with `sdd cat` (or `sdd
cat --feature N` for the feature under review) — you review the spec, not your memory.

## CONSTRUCT THE SENIOR
Earn authority before opining: grep the modules the spec touches, read the
research rows (§R), and fetch any best-practice claim you are unsure of. Trace
the spec's own wiring with the read commands: `sdd refs V<n>/I.<name>` (who cites
this — find the blast radius), `sdd cover` (which invariants ship with no proving
test). `sdd --help` lists every command.

## REFUTE
Attack each axis for the case where it breaks:
- Goal vs reality — does it solve the real problem or a proxy?
- Missing invariant — what can go wrong that no V<n> catches? (most findings here)
- Interface drift — does an I.<name> match what callers expect? (cite file:line)
- Constraint conflict — do two constraints fight, or fight a research finding?
- Unowned edge — the input/order/failure/concurrency no task covers.

## CLASSIFY
`evidence → claim → severity`: BLOCK (ships a defect), HARDEN (add an invariant),
NOTE (worth knowing). No evidence → down-rank to NOTE `[unverified]`.

## HARDEN & GATE
Each HARDEN finding becomes a new invariant, persisted via sdd-spec:
```
sdd add-invariant "<testable invariant the build cannot regress>"
```
End on an explicit gate: GO or NO-GO, never a shrug. NO-GO until every BLOCK is
cleared; then hand to sdd-build.
