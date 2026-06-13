#!/usr/bin/env bash
set -euo pipefail

# --- OS detection ---
case "$(uname -s)" in
  Linux)  OS=linux ;;
  Darwin) OS=darwin ;;
  *)
    echo "Error: unsupported operating system: $(uname -s)" >&2
    exit 1
    ;;
esac

# --- Architecture detection ---
case "$(uname -m)" in
  x86_64)          ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  *)
    echo "Error: unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

# --- Version resolution ---
# Use FORGE_VERSION env var if set; otherwise query GitHub for the latest release.
# The || true prevents set -e from aborting if the API returns 404 (no releases yet)
# or if grep finds no tag_name in the response.
FORGE_VERSION=${FORGE_VERSION:-$(
  curl -fsSL https://api.github.com/repos/kanukuntla-R/forge/releases/latest \
    | grep '"tag_name"' | head -n1 | cut -d'"' -f4 || true
)}

if [ -z "$FORGE_VERSION" ]; then
  echo "Error: could not determine forge version." >&2
  echo "Set FORGE_VERSION to install a specific version, or check that releases exist at:" >&2
  echo "  https://github.com/kanukuntla-R/forge/releases" >&2
  exit 1
fi

# --- Download ---
URL="https://github.com/kanukuntla-R/forge/releases/download/${FORGE_VERSION}/forge-${OS}-${ARCH}"

mkdir -p ~/.local/bin

TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT

if ! curl -fsSL -o "$TMP" "$URL"; then
  echo "Error: failed to download from $URL" >&2
  echo "If this is the first time installing, check that releases exist at:" >&2
  echo "  https://github.com/kanukuntla-R/forge/releases" >&2
  exit 1
fi
chmod +x "$TMP"
mv "$TMP" ~/.local/bin/forge

# --- PATH check ---
if ! echo "$PATH" | tr ':' '\n' | grep -Fxq "$HOME/.local/bin"; then
  echo ""
  echo "Note: ~/.local/bin is not on your PATH."
  echo "Add this to your shell config (e.g., ~/.bashrc or ~/.zshrc):"
  echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

# --- Done ---
echo ""
echo "✓ Installed forge $FORGE_VERSION to ~/.local/bin/forge"
echo ""
echo "Try it out:"
echo "  forge --help"
echo "  forge list"
