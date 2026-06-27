#!/usr/bin/env bash
set -euo pipefail

# --- Color setup ---
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  BLUE='\033[38;5;33m'
  ORANGE='\033[38;5;208m'
  GREEN='\033[38;5;42m'
  GRAY='\033[38;5;245m'
  BOLD='\033[1m'
  RESET='\033[0m'
else
  BLUE='' ORANGE='' GREEN='' GRAY='' BOLD='' RESET=''
fi

print_banner() {
  printf "${BLUE}${BOLD}"
  cat << 'EOF'
███████╗ ██████╗ ██████╗  ██████╗ ███████╗
██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝
█████╗  ██║   ██║██████╔╝██║  ███╗█████╗
██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝
██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗
╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝
EOF
  printf "${RESET}"
  printf "  ${ORANGE}Scaffold projects you actually want to build${RESET}\n\n"
}

print_banner

# --- [1/4] Platform detection ---
printf "${ORANGE}[1/4]${RESET} Detecting platform...\n"

case "$(uname -s)" in
  Linux)  OS=linux ;;
  Darwin) OS=darwin ;;
  *)
    echo "Error: unsupported operating system: $(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64)          ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  *)
    echo "Error: unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

printf "      ${GRAY}OS: $OS, Arch: $ARCH${RESET}\n"

# --- [2/4] Version resolution ---
printf "${ORANGE}[2/4]${RESET} Resolving version...\n"

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

printf "      ${GRAY}Version: $FORGE_VERSION${RESET}\n"

# --- [3/4] Download ---
printf "${ORANGE}[3/4]${RESET} Downloading...\n"

URL="https://github.com/kanukuntla-R/forge/releases/download/${FORGE_VERSION}/forge-${OS}-${ARCH}"
printf "      ${GRAY}$URL${RESET}\n"

mkdir -p ~/.local/bin

TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT

if ! curl -fsSL -o "$TMP" "$URL"; then
  echo "Error: failed to download from $URL" >&2
  echo "If this is the first time installing, check that releases exist at:" >&2
  echo "  https://github.com/kanukuntla-R/forge/releases" >&2
  exit 1
fi

# --- [4/4] Install ---
printf "${ORANGE}[4/4]${RESET} Installing...\n"

chmod +x "$TMP"
mv "$TMP" ~/.local/bin/forge

# --- PATH check ---
if ! echo "$PATH" | tr ':' '\n' | grep -Fxq "$HOME/.local/bin"; then
  printf "\n${GRAY}Note: ~/.local/bin is not on your PATH.${RESET}\n"
  printf "${GRAY}Add this to your shell config (e.g., ~/.bashrc or ~/.zshrc):${RESET}\n"
  printf "  ${ORANGE}export PATH=\"\$HOME/.local/bin:\$PATH\"${RESET}\n"
fi

# --- Done ---
printf "\n${GREEN}✓${RESET} Installed forge ${BOLD}$FORGE_VERSION${RESET} to ~/.local/bin/forge\n\n"
printf "Try it out:\n"
printf "  ${GRAY}forge --help${RESET}\n"
printf "  ${GRAY}forge list${RESET}\n"
