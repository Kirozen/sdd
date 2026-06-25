---
name: sdd-grill
description: |
  Sharpen a fuzzy idea into a feature's goal + constraints before any task
  exists, persisting answers into spec.db via the sdd CLI. One question at a
  time, each with a recommended answer; unknowns parked, never guessed. Triggers
  when the user has a vague idea, says "grill me", "stress-test this", or invokes
  /sdd-grill. Defers durable writes to sdd-spec.
---

# sdd-grill — sharpen idea before tasks exist

**One question at a time. Every answer lands in spec.db or is parked `?`. Never guess a constraint.**

A bad assumption caught here costs one question; caught after build it costs a bug.

## CALIBRATE
One opening read: how well does the user know the domain, how locked is the
idea, how much pressure (light / normal / brutal). Match it.

## QUESTION LADDER
Climb in order; ask **one**, **recommend** an answer, wait:
1. Goal — what must the code do, in one line?
2. Done — the observable that proves it works?
3. Boundary — what is explicitly out of scope?
4. Lock — what tech/lib is non-negotiable or forbidden?
5. Surface — what does the outside world touch (cmd/api/file/env)?
6. Edge — the one input that breaks the happy path?
7. Unknown — what do we not know yet? → park `?`.

Stop the moment the spec would be unambiguous.

## PERSIST (via sdd, not files)
Open or create the feature, then land each settled answer:
```
fid=$(sdd new-feature "<short-name>")
sdd add-goal "<goal line>" --feature $fid
sdd add-constraint "<constraint>" --feature $fid
```
Durable truths (invariants/interfaces) wait for sdd-spec — grill proposes, spec
writes. Never hand-edit SPEC.md; it is generated (run `sdd cat` to read it).

## HANDOFF
When §G is one line with one reading and every blocking unknown is answered or
parked `?`, hand the feature id to **sdd-spec** to draft invariants/interfaces
and break the goal into tasks.
