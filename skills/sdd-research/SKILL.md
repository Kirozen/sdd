---
name: sdd-research
description: |
  Gather external knowledge a spec decision needs and persist it into the durable
  research log via the sdd CLI, so build grounds in facts not hallucination. Each
  finding cites a source; unsourced claims are flagged `?`, never written as fact.
  Triggers when a decision hinges on a lib/API/best-practice, "research this",
  "what's the best lib for…", or /sdd-research.
---

# sdd-research — external knowledge → durable log

**Every finding cites a source. No source → flag `?`, never write a guess as fact.**

Research is the external oracle: pull the real fact once, log it, never
re-derive. Research rows are durable in spec.db — they survive feature wipes.

## FOUR STEPS
1. SCOPE — turn the unknown into 1-3 concrete questions. Vague "research auth" →
   "JWT lib for Node ESM, maintained?".
2. GATHER — prefer primary sources (official docs, the repo, the RFC). For a big
   sweep, spawn a sub-agent so raw pages never touch this context.
3. DISTILL — crush each answer to one terse line + its source.
4. PERSIST — write each finding through the CLI:
   ```
   sdd add-research "<topic>" "<one-line finding>" "<source url>"
   ```

## SOURCE DISCIPLINE
- Cite a URL/repo/RFC/paper per finding. Verbatim identifiers and versions.
- Could not verify → still record it but flag `?` in the finding text.
- Conflicting sources → record both, let the user pick. Never silently average.

## STOP
Done when every scoped question has a sourced row (or an honest `?`) and no build
decision rests on an unchecked assumption.
