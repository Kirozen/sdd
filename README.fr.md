# sdd — développement piloté par la spécification

`sdd` est un moteur de spécification adossé à SQLite. La spec vit dans une base
de données ; `SPEC.md` n'en est qu'une **vue Markdown générée** (jamais éditée à
la main). Chaque mutation ré-exporte `SPEC.md` de façon atomique, si bien que le
fichier et la base ne divergent jamais (`sdd check` le garantit).

## Pourquoi

Écrire le code avant d'avoir tranché le *quoi* et le *pourquoi* coûte cher : une
mauvaise hypothèse attrapée au moment de la spec coûte une question ; attrapée
après le build, elle coûte un bug. `sdd` rend la spec **interrogeable,
versionnable et infalsifiable** :

- une coupe nette entre le **durable** (invariants, interfaces, bugs, recherche —
  survit aux features) et l'**éphémère** (goals, contraintes, tâches d'une
  feature — effacé avec elle) ;
- des **citations à clés réelles** (`V<n>`, `I.<name>`) protégées par des clés
  étrangères : une tâche ne peut pas citer un invariant inexistant ;
- une base **globale** partagée par tous les dépôts, chaque projet étant isolé.

## Le workflow (skills)

Les skills `sdd-*` orchestrent le cycle de vie ; chaque écriture durable passe
par la CLI `sdd`, jamais par une édition manuelle de `SPEC.md`.

```
grill → spec → research → review → build → backprop → deepen
```

- **sdd-grill** — affûte une idée floue en goal + contraintes. Une question à la
  fois ; les inconnues sont parquées (`add-unknown`), jamais devinées.
- **sdd-spec** — seul mutateur de la spec : invariants, interfaces, tâches.
- **sdd-research** — rassemble des faits externes ; chaque trouvaille cite sa source.
- **sdd-review** — revue adversariale : tente de réfuter la spec avant tout code,
  finit sur un gate go / no-go.
- **sdd-build** — implémente tâche par tâche ; bascule le statut via `set-task`.
- **sdd-backprop** — bug → invariant : sur un échec, décide si un nouvel invariant
  empêcherait la récurrence.
- **sdd-deepen** — passe optionnelle d'amélioration du design (budget restant).

Pour savoir où vous en êtes : `sdd guide` indique, par feature, l'étape inférée
et la skill suivante recommandée ; `sdd next` donne la prochaine tâche
actionnable avec son goal et ses citations résolues.

## Commandes

**Cycle de vie** : `init`, `export`, `check`, `backup`, `import`

**Mutations éphémères (par feature)** : `new-feature`, `add-goal`,
`add-constraint`, `add-task`, `set-task`, `wipe-feature`, `add-unknown`,
`resolve-unknown`

**Mutations durables** : `add-invariant`, `add-interface`, `add-bug`,
`add-research`, `edit`, `deprecate-interface`

**Lectures (pures, sans ré-export)** : `show`, `list` (avec `--pretty`, et pour
les tâches `--status`/`--feature`), `refs`, `status`, `next`, `guide`

Statuts de tâche : `.` à faire · `~` en cours · `x` fait.
Statuts d'unknown : `open` · `resolved` (jamais supprimé).

## Où vivent les données

La base globale est unique, hors de tout dépôt :

```
$XDG_CONFIG_HOME/sdd/spec.db   (par défaut ~/.config/sdd/spec.db)
```

Elle est créée à la première commande et migrée automatiquement lors d'une montée
de schéma. Un projet est identifié par l'URL de son remote git (à défaut, le
chemin de sa worktree principale) : clones et worktrees d'un même dépôt
partagent la même spec. `SPEC.md` et `spec.db` locaux sont gitignorés ; ce
README, lui, est versionné.
