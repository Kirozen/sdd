#!/bin/sh
# ensure-sdd-binary.sh — provision the `sdd` CLI for the Claude plugin (F16 T89).
#
# Invoked by the SessionStart hook (hooks/hooks.json). Downloads the prebuilt
# release binary matching the plugin version into ${CLAUDE_PLUGIN_ROOT}/bin/sdd,
# which Claude Code adds to the Bash tool PATH (R17) so skills invoke `sdd` bare.
#
# Invariants enforced here:
#   V81  binary lands in ${CLAUDE_PLUGIN_ROOT}/bin/sdd (auto-PATH); no hardcoded path elsewhere
#   V82  release tag fetched == plugin.json.version (binary coupled to plugin version)
#   V83  atomic + idempotent: no-op if present; download->verify->rename; no partial binary
#   V84  SHA256 verified against published SHA256SUMS BEFORE placing on PATH
#   V85  OS/arch via uname; only darwin/linux x amd64/arm64; else abort
#   V88  NEVER fatal to the session: every failure warns on stderr and exits 0
#   V89  on any failure, print manual-install instructions (never a silent break)
#   V90  explicit, total uname->asset mapping
#
# POSIX sh; no bashisms. Testable via SDD_RELEASE_BASE_URL override.

set -u

REPO="Kirozen/sdd"
# Test seam: override the release base URL (e.g. file:///fixtures) to exercise
# the download/verify/install path offline. Defaults to GitHub Releases.
BASE_URL_OVERRIDE="${SDD_RELEASE_BASE_URL:-}"

warn() { printf '[sdd-plugin] %s\n' "$1" >&2; }

# V89: manual fallback, then ALWAYS exit 0 (V88 — provisioning never bricks a session).
give_up() {
	warn "$1"
	warn "sdd binary not provisioned automatically. Install it manually:"
	warn "  go install github.com/kirozen/sdd@latest   # if you have Go"
	warn "  # or download a binary from https://github.com/${REPO}/releases"
	warn "Then ensure it is on your PATH. The plugin's sdd-* skills need it."
	exit 0
}

# Must run inside the plugin (R18). No root -> nothing to provision.
[ -n "${CLAUDE_PLUGIN_ROOT:-}" ] || give_up "CLAUDE_PLUGIN_ROOT unset (not in plugin context)."

BIN_DIR="${CLAUDE_PLUGIN_ROOT}/bin"
BIN="${BIN_DIR}/sdd"

# V83: idempotent no-op if already provisioned and executable.
if [ -x "$BIN" ]; then
	exit 0
fi

# V90: explicit, total uname -> goreleaser os/arch mapping.
uname_s="$(uname -s 2>/dev/null || echo unknown)"
uname_m="$(uname -m 2>/dev/null || echo unknown)"
case "$uname_s" in
	Darwin) OS=darwin ;;
	Linux)  OS=linux ;;
	*) give_up "unsupported OS '${uname_s}' (Windows native is out of scope; macOS/Linux/WSL only)." ;;
esac
case "$uname_m" in
	x86_64|amd64)  ARCH=amd64 ;;
	aarch64|arm64) ARCH=arm64 ;;
	*) give_up "unsupported arch '${uname_m}' (need x86_64/amd64 or aarch64/arm64)." ;;
esac
ASSET="sdd_${OS}_${ARCH}"

# V82: tag == plugin.json.version. Parse without jq.
MANIFEST="${CLAUDE_PLUGIN_ROOT}/.claude-plugin/plugin.json"
[ -f "$MANIFEST" ] || give_up "manifest not found at ${MANIFEST}."
VERSION="$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$MANIFEST" | head -1)"
[ -n "$VERSION" ] || give_up "could not read version from ${MANIFEST}."

if [ -n "$BASE_URL_OVERRIDE" ]; then
	BASE="$BASE_URL_OVERRIDE"
else
	BASE="https://github.com/${REPO}/releases/download/v${VERSION}"
fi

# Downloader: curl or wget. -> "$1" url, "$2" dest.
if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -q "$1" -O "$2"; }
else
	give_up "neither curl nor wget available to download the binary."
fi

# Checksum tool: sha256sum (Linux) or shasum -a 256 (macOS). -> prints hex.
if command -v sha256sum >/dev/null 2>&1; then
	sha256() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
	sha256() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
	give_up "no sha256 tool (sha256sum/shasum) for integrity check (V84)."
fi

# Stage in the bin dir itself so the final rename is atomic (same filesystem, V83).
mkdir -p "$BIN_DIR" 2>/dev/null || give_up "cannot create ${BIN_DIR} (not writable)."
TMP_BIN="${BIN}.tmp.$$"
TMP_SUMS="${BIN_DIR}/.SHA256SUMS.$$"
cleanup() { rm -f "$TMP_BIN" "$TMP_SUMS"; }
trap cleanup EXIT INT TERM

fetch "${BASE}/${ASSET}" "$TMP_BIN" || give_up "download failed: ${BASE}/${ASSET}"
fetch "${BASE}/SHA256SUMS" "$TMP_SUMS" || give_up "download failed: ${BASE}/SHA256SUMS"

# V84: verify BEFORE placing on PATH. SHA256SUMS line: "<hex>  sdd_<os>_<arch>".
EXPECTED="$(grep " ${ASSET}\$" "$TMP_SUMS" | head -1 | cut -d' ' -f1)"
[ -n "$EXPECTED" ] || give_up "no checksum entry for ${ASSET} in SHA256SUMS."
ACTUAL="$(sha256 "$TMP_BIN")"
if [ "$EXPECTED" != "$ACTUAL" ]; then
	give_up "checksum mismatch for ${ASSET} (expected ${EXPECTED}, got ${ACTUAL}) — refusing to install."
fi

chmod +x "$TMP_BIN" 2>/dev/null || give_up "cannot chmod the downloaded binary."
# Atomic publish onto PATH (V83): rename within the same directory.
mv -f "$TMP_BIN" "$BIN" || give_up "cannot install binary to ${BIN}."
rm -f "$TMP_SUMS"
trap - EXIT INT TERM
warn "provisioned sdd ${VERSION} (${OS}/${ARCH}) -> ${BIN}"
exit 0
