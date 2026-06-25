# Makefile — sdd dev tooling (F15). Wraps the CLI build, the project gate, and the
# pinned `go tool` helpers. All tools (sqlc, gofumpt, betteralign, go-mod-upgrade)
# are pinned via the `tool` directive in go.mod and run through `go tool`, never a
# global install, so versions stay reproducible.

.PHONY: build test vet gen fmt align upgrade gate

build:
	go build -o sdd .

test:
	go test ./...

vet:
	go vet ./...

gen:
	go tool sqlc generate

# fmt/align act ONLY on the sole hand-written package (root `.`); the generated
# db/ package is excluded by construction (V78). Safe forms only: `gofumpt -w
# *.go` (root glob, non-recursive) and `betteralign -apply .` (root package, NOT
# ./...). Reformatting db/ would diverge from fresh sqlc codegen and break the
# gate (V54).
fmt:
	go tool gofumpt -w *.go

# betteralign is a go/analysis tool: it applies the safe reorders but still exits
# non-zero (3) while any finding remains — including structs it cannot auto-fix
# (positional composite literals). That exit is advisory, so tolerate it; the gate
# never depends on it (V79).
align:
	go tool betteralign -apply . || true

# Interactive TUI — manual only, kept out of the gate.
upgrade:
	go tool go-mod-upgrade

# Non-interactive gate, mirrors CLAUDE.md: build + vet + test + codegen
# reproducibility (`sqlc generate` must leave the tree byte-clean, V54).
gate: build vet test gen
	git diff --exit-code
