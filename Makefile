# Makefile — sdd dev tooling (F15). Wraps the CLI build, the project gate, and the
# pinned `go tool` helpers. All tools (sqlc, gofumpt, betteralign, go-mod-upgrade)
# are pinned via the `tool` directive in go.mod and run through `go tool`, never a
# global install, so versions stay reproducible.

.PHONY: build test vet gen fmt align upgrade gate

build:
	CGO_ENABLED=0 go build -trimpath -o sdd .

test:
	go tool gotestsum --format-hide-empty-pkg ./...

vet:
	go vet ./...

gen:
	go tool sqlc generate

# fmt/align never reach the generated db/ package: gofumpt and betteralign both
# skip files marked `// Code generated ... DO NOT EDIT.`, so the invocation form
# (recursive or scoped) is irrelevant to db/ (V78). The gate's git diff is the
# real backstop (V54).
fmt:
	go tool gofumpt -l -w -extra .

# betteralign is a go/analysis tool: it applies the safe reorders but still exits
# non-zero (3) while any finding remains — including structs it cannot auto-fix
# (positional composite literals). That exit is advisory, so tolerate it; the gate
# never depends on it (V79).
align:
	go tool betteralign -apply . || true

# Interactive TUI — manual only, kept out of the gate.
upgrade:
	go tool go-mod-upgrade
	go mod tidy

# Non-interactive gate, mirrors CLAUDE.md: build + vet + test + codegen
# reproducibility (`sqlc generate` must leave the tree byte-clean, V54).
gate: build vet test gen
	git diff --exit-code

before-commit: gen align fmt test vet build