# sdd — développement piloté par la spécification

*[English version](README.md).*

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

## Les éléments d'une spec

Une spec `sdd` est faite de quelques **briques** typées. Chacune a une raison
d'être précise ; c'est ce vocabulaire qui rend la spec interrogeable. Si vous ne
deviez retenir qu'une chose, ce serait la coupe ci-dessous.

### La coupe fondamentale : durable vs éphémère

- Le **durable** est la mémoire long terme du projet : ce qui reste vrai d'une
  feature à l'autre. On ne l'efface jamais en routine.
- L'**éphémère** appartient à une **feature** — une unité de travail. Quand la
  feature est terminée ou abandonnée, tout son éphémère est effacé ; le durable,
  lui, survit. C'est ce qui empêche la spec de gonfler indéfiniment.

### Les briques durables

| Brique | Clé | Ce que c'est |
|---|---|---|
| **Invariant** | `V<n>` | Une règle qui doit *toujours* rester vraie, écrite pour être testable. Ex. « une tâche ne peut pas citer un invariant inexistant ». La mémoire des décisions prises. |
| **Interface** | `I.<nom>` | Un point de contact stable avec l'extérieur (une commande, une API, un fichier) et sa signature. Ex. `I.export` = `sdd export → régénère SPEC.md`. |
| **Bug** | `B<n>` | La trace d'une erreur passée, reliée à l'invariant qui empêche son retour. La leçon apprise, gravée pour de bon. |
| **Research** | `R<n>` | Un fait externe vérifié (doc officielle, RFC…) avec sa source. Pour fonder une décision sur un fait, pas sur une supposition. |
| **Test** | — | Le lien *déclaré* entre un invariant et le test qui le prouve. Rend la promesse « ce code ne peut plus régresser » vérifiable (`sdd cover`). |

### Les briques éphémères (portées par une feature)

| Brique | Clé | Ce que c'est |
|---|---|---|
| **Feature** | `F<n>` | Une unité de travail : un objectif et tout ce qu'il faut pour l'atteindre. |
| **Goal** | `§G` | L'objectif de la feature en une phrase : ce que le code doit faire. |
| **Constraint** | `§C` | Une limite ou une exigence non négociable : hors-scope, techno imposée, contrat à respecter. |
| **Task** | `T<n>` | Une étape d'implémentation concrète, avec un statut et les briques durables qu'elle cite. |
| **Unknown** | `U<n>` | Une question encore ouverte, *parquée* plutôt que devinée. Passe de `open` à `resolved`. |
| **Gate** | — | Le verdict de review de la feature (`go` / `no-go`) : a-t-elle passé l'examen adversarial avant le build ? |

### Les citations — le ciment

Une tâche déclare de quoi elle dépend :
`sdd add-task "…" --cites V2,I.export`. Des clés étrangères interdisent de citer
un invariant ou une interface qui n'existe pas — impossible de référencer du
vide. C'est ce qui rend la spec **infalsifiable** : `sdd refs V2` montre en
retour tout ce qui dépend de `V2`, donc rien ne casse en silence quand on y
touche.

### Les clés `V<n>` / `T<n>` …

Chaque brique numérotée porte un **ordinal** propre au projet : le premier
invariant est `V1`, le deuxième `V2`, etc. (idem `T`, `B`, `R`, `U`, `F`). C'est
cette clé courte que vous lisez dans `SPEC.md` et que vous passez aux commandes
(`sdd show V2`, `sdd set-task 7 --status x`).

## Installation comme plugin Claude Code

`sdd` se distribue comme **plugin Claude Code** : les skills et les slash-commands
`/sdd:*` sont packagés, et le binaire `sdd` est **provisionné automatiquement** au
démarrage de session.

```
/plugin marketplace add Kirozen/sdd
/plugin install sdd@sdd-marketplace
```

Au premier `SessionStart`, un hook détecte votre OS/arch et télécharge le binaire
`sdd` correspondant depuis la [release GitHub](https://github.com/Kirozen/sdd/releases)
de la version du plugin, vérifie son **SHA256**, puis l'installe dans le `bin/` du
plugin (ajouté au `PATH` de l'outil Bash) — les skills appellent donc `sdd` sans
configuration. Le provisioning est **idempotent** (ne retélécharge pas si présent)
et **jamais bloquant** : en cas d'échec (réseau, plateforme non gérée) il affiche
des instructions d'installation manuelle et laisse la session continuer.

**Plateformes supportées** : macOS et Linux (dont WSL), sur `amd64` et `arm64`.
Windows natif n'est pas géré en v1 — sous Windows, utilisez WSL ou installez le
binaire à la main (`go install github.com/kirozen/sdd/cmd/sdd@latest`).

> **Dogfooding du dépôt lui-même** — ce dépôt *est* le plugin (les skills vivent
> sous `skills/`, plus sous `.claude/skills/` : source unique). Pour travailler
> sur `sdd` avec ses propres skills, installez le plugin localement :
> `/plugin marketplace add ./` puis `/plugin install sdd@sdd-marketplace`.

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
  finit sur un verdict go / no-go enregistré par `sdd gate` (que `sdd guide`
  relit ensuite).
- **sdd-build** — implémente tâche par tâche ; bascule le statut via `set-task`,
  relie chaque test à l'invariant qu'il prouve via `sdd add-test` (vérifiable
  avec `sdd cover`).
- **sdd-backprop** — bug → invariant : sur un échec, décide si un nouvel invariant
  empêcherait la récurrence.
- **sdd-deepen** — passe optionnelle d'amélioration du design (budget restant).

Pour savoir où vous en êtes : `sdd guide` indique, par feature, l'étape inférée
et la skill suivante recommandée ; `sdd next` donne la prochaine tâche
actionnable avec son goal et ses citations résolues.

## Commandes

**Cycle de vie** : `init`, `export`, `check`, `backup`, `import`

`sdd backup [chemin]` snapshote tout le store global (VACUUM INTO) ; sans
argument il écrit un fichier horodaté `spec-backup-<date>.db` dans le dossier
courant et affiche son chemin (`--sql` pour un dump texte portable).

**Mutations éphémères (par feature)** : `new-feature`, `add-goal`,
`add-constraint`, `add-task`, `add-cite`, `set-task`, `wipe-feature`,
`add-unknown`, `resolve-unknown`, `gate`, `rm-task`, `rm-goal`, `rm-constraint`

**Mutations durables** : `add-invariant`, `add-interface`, `add-bug`,
`add-research`, `add-test`, `edit`, `deprecate-interface`, `retract-invariant`,
`retract-interface`

**Batch** : `sdd apply` lit sur stdin des sous-commandes `add-*` TAB-délimitées
(une par ligne) et les applique **dans une seule transaction** — tout-ou-rien,
un seul ré-export final ; un `new-feature` en tête fixe la feature courante.
C'est le levier d'écriture groupée des agents (sdd-spec).

**Lectures (pures, sans ré-export)** : `show`, `list` (avec `--pretty`, et pour
les tâches `--status`/`--feature`), `refs`, `status`, `next`, `todo`, `guide`,
`cover`, `search`, `projects`, `stats`

Statuts de tâche : `.` à faire · `~` en cours · `x` fait.
Statuts d'unknown : `open` · `resolved` (jamais supprimé).
Verdicts de gate : `go` · `no-go` (un seul par feature, le dernier remplace).

**La rétraction** (`retract-invariant`, `retract-interface`, `rm-task`,
`rm-goal`, `rm-constraint`) supprime *réellement* une ligne — contrairement à
`deprecate-interface` qui ne fait que marquer. Une ligne durable **citée** ne
peut pas être retirée (la commande refuse en listant ses citants) ; on retire
d'abord ce qui la cite. `retract-invariant` prévient quand il emporte au passage
les tests qui le prouvent. `rm-goal`/`rm-constraint` ciblent la *n*-ième ligne
d'une feature (`sdd rm-goal <F-ord> <n>`, 1-based, dans l'ordre affiché).

`edit` amende le texte d'une ligne en place (l'id ne bouge pas, les citations
restent valides). Les goals et constraints s'adressent **par position**, comme la
rétraction : `sdd edit goal <F-ord> <n> --text "…"` (idem `constraint`), jamais
par PK global — un id d'un autre projet du store partagé est ainsi inatteignable.
Les autres kinds gardent leur clé : `sdd edit <kind> <ord|nom> --text "…"`. Pour
voir ces positions avant d'éditer, `sdd list goal` / `sdd list constraint` listent
les lignes du projet courant sous la forme `F<ord> <n> | texte`.

`add-cite <T-ord> <cite>…` attache des citations `V<n>`/`I.<name>` à une tâche
*existante* sans la recréer (FK-gardée, V5) — le complément de `add-task --cites`
quand on cite après coup.

Quelques commandes utiles pour s'y retrouver : `sdd guide` (où en est chaque
feature, et quelle skill lancer ensuite), `sdd next` (la prochaine tâche
actionnable, avec son goal et ses citations résolues), `sdd todo` (toutes les
tâches non finies en TSV — contrat de colonnes stable pour scripts et agents ;
`--pretty` pour une vue humaine groupée), `sdd cover` (quels
invariants sont gardés par un test, lesquels ne le sont pas), `sdd search
<terme>` (recherche plein-texte sur le *contenu* des lignes du projet courant —
là où `refs` cherche par clé de citation), `sdd projects` (tous les projets du
store global avec leurs compteurs ; la seule commande qui regarde au-delà du
projet courant), `sdd stats` (compteurs de volume par type — invariants,
interfaces, bugs, research, tests, unknowns, features, tâches ventilées par
statut — pour le projet courant ; `sdd stats --all` agrège tout le store et
ajoute le nombre de projets et la taille du fichier `spec.db`).

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
