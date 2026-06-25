#!/bin/sh
# Test oracle for ensure-sdd-binary.sh (F16 T89). POSIX sh.
# Exercises V81/V82/V83/V84/V85/V88/V89/V90 via the SDD_RELEASE_BASE_URL seam.
set -u
HERE="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="${HERE}/ensure-sdd-binary.sh"
PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); printf '  ok   %s\n' "$1"; }
bad()  { FAIL=$((FAIL+1)); printf '  FAIL %s\n' "$1"; }

sha() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1"|cut -d' ' -f1; else shasum -a 256 "$1"|cut -d' ' -f1; fi; }

# Fresh plugin root with a manifest at version 0.1.0.
new_root() {
	r="$(mktemp -d)"; mkdir -p "$r/.claude-plugin"
	printf '{\n  "name": "sdd",\n  "version": "0.1.0"\n}\n' > "$r/.claude-plugin/plugin.json"
	echo "$r"
}
# Fixture release dir holding a fake binary + valid SHA256SUMS for this host.
new_release() {
	d="$(mktemp -d)"
	printf '#!/bin/sh\necho FAKE_SDD\n' > "$d/sdd_linux_amd64"
	printf "%s  sdd_linux_amd64\n" "$(sha "$d/sdd_linux_amd64")" > "$d/SHA256SUMS"
	echo "$d"
}

echo "== T89 ensure-sdd-binary.sh =="

# 1. Happy path (V81/V83/V84): downloads, verifies, installs executable on PATH.
ROOT="$(new_root)"; REL="$(new_release)"
CLAUDE_PLUGIN_ROOT="$ROOT" SDD_RELEASE_BASE_URL="file://$REL" sh "$SCRIPT" 2>/dev/null
if [ -x "$ROOT/bin/sdd" ] && [ "$("$ROOT/bin/sdd")" = "FAKE_SDD" ]; then ok "happy path installs ${ROOT##*/}/bin/sdd (V81)"; else bad "happy path"; fi

# 2. Idempotent no-op (V83): second run leaves a sentinel binary untouched.
printf '#!/bin/sh\necho SENTINEL\n' > "$ROOT/bin/sdd"; chmod +x "$ROOT/bin/sdd"
CLAUDE_PLUGIN_ROOT="$ROOT" SDD_RELEASE_BASE_URL="file://$REL" sh "$SCRIPT" 2>/dev/null
[ "$("$ROOT/bin/sdd")" = "SENTINEL" ] && ok "idempotent no-op when present (V83)" || bad "idempotent no-op"

# 3. Checksum mismatch (V84): tampered binary -> NOT installed, exit 0, no partial.
ROOT="$(new_root)"; REL="$(new_release)"
printf '#!/bin/sh\necho TAMPERED\n' > "$REL/sdd_linux_amd64"   # content now != SHA256SUMS
CLAUDE_PLUGIN_ROOT="$ROOT" SDD_RELEASE_BASE_URL="file://$REL" sh "$SCRIPT" 2>/dev/null
rc=$?
{ [ ! -e "$ROOT/bin/sdd" ] && [ "$rc" -eq 0 ]; } && ok "checksum mismatch refuses install + exit 0 (V84/V88)" || bad "checksum mismatch (rc=$rc)"
ls "$ROOT/bin/"sdd.tmp.* >/dev/null 2>&1 && bad "left a partial binary" || ok "no partial binary on failure (V83)"

# 4. Missing release (V88/V89): bad URL -> exit 0, no binary, prints manual hint.
ROOT="$(new_root)"
out="$(CLAUDE_PLUGIN_ROOT="$ROOT" SDD_RELEASE_BASE_URL="file:///nonexistent_dir_xyz" sh "$SCRIPT" 2>&1)"; rc=$?
{ [ "$rc" -eq 0 ] && [ ! -e "$ROOT/bin/sdd" ]; } && ok "download failure non-fatal exit 0 (V88)" || bad "download failure (rc=$rc)"
printf '%s' "$out" | grep -q "go install" && ok "prints manual-install fallback (V89)" || bad "no manual fallback"

# 5. Unset CLAUDE_PLUGIN_ROOT (V88): exit 0, no crash.
( unset CLAUDE_PLUGIN_ROOT; sh "$SCRIPT" >/dev/null 2>&1 ); [ $? -eq 0 ] && ok "unset root -> exit 0 (V88)" || bad "unset root"

# 6. Unsupported arch path (V85/V90): no SHA256SUMS entry for a foreign asset.
#    Simulate by a release missing this host's asset -> give_up, exit 0, no install.
ROOT="$(new_root)"; REL="$(mktemp -d)"; : > "$REL/SHA256SUMS"
out="$(CLAUDE_PLUGIN_ROOT="$ROOT" SDD_RELEASE_BASE_URL="file://$REL" sh "$SCRIPT" 2>&1)"; rc=$?
{ [ "$rc" -eq 0 ] && [ ! -e "$ROOT/bin/sdd" ]; } && ok "missing asset/checksum entry -> safe exit 0 (V84/V88)" || bad "missing asset (rc=$rc)"

echo "== $PASS passed, $FAIL failed =="
[ "$FAIL" -eq 0 ]
