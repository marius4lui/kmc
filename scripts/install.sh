#!/bin/sh

set -eu

PACKAGE_NAME="@marius4lui/kmc"
MIN_NODE_MAJOR=18
DEFAULT_USER_PREFIX="${HOME:-}/.local"

COMMAND="install"
VERSION="latest"
PREFIX=""
MODIFY_PATH=0
QUIET=0
STEP=0
TOTAL_STEPS=6

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  GREEN="$(printf '\033[32m')"
  CYAN="$(printf '\033[36m')"
  DIM="$(printf '\033[2m')"
  BOLD="$(printf '\033[1m')"
  RESET="$(printf '\033[0m')"
else
  GREEN=""
  CYAN=""
  DIM=""
  BOLD=""
  RESET=""
fi

say() {
  if [ "$QUIET" -eq 0 ]; then
    printf '%s\n' "$*"
  fi
}

step() {
  STEP=$((STEP + 1))
  if [ "$QUIET" -eq 0 ]; then
    printf '\n%s[%s/%s]%s %s%s%s\n' "$CYAN" "$STEP" "$TOTAL_STEPS" "$RESET" "$BOLD" "$1" "$RESET"
    [ -z "${2:-}" ] || printf '      %s$ %s%s\n' "$DIM" "$2" "$RESET"
  fi
}

pass() {
  say "      ${GREEN}✓${RESET} $*"
}

fail() {
  printf 'kmc installer: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Install and manage kmc.

Usage:
  install.sh [install|update|uninstall|doctor] [options]

Options:
  --version VERSION  Install a specific npm version (default: latest)
  --prefix PATH      Install below PATH instead of the npm global prefix
  --modify-path      Add the selected bin directory to your shell profile
  --quiet            Only print errors
  -h, --help         Show this help

Examples:
  curl -fsSL https://kmc.kmuc.app/install.sh | sh
  curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- update
  curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- --version 1.0.10
  curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- uninstall
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    install|update|uninstall|doctor)
      COMMAND="$1"
      ;;
    --version)
      [ "$#" -ge 2 ] || fail "--version requires a value"
      VERSION="$2"
      shift
      ;;
    --prefix)
      [ "$#" -ge 2 ] || fail "--prefix requires a path"
      PREFIX="$2"
      shift
      ;;
    --modify-path)
      MODIFY_PATH=1
      ;;
    --quiet)
      QUIET=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1 (use --help)"
      ;;
  esac
  shift
done

case "$COMMAND" in
  uninstall|doctor) TOTAL_STEPS=5 ;;
esac

case "$(uname -s 2>/dev/null || printf unknown)" in
  Linux*) SYSTEM_NAME="Linux" ;;
  Darwin*) SYSTEM_NAME="macOS" ;;
  *) SYSTEM_NAME="$(uname -s 2>/dev/null || printf unknown)" ;;
esac
step "Detect system" "uname -s"
pass "$SYSTEM_NAME detected"

step "Check Node.js and npm" "node --version && npm --version"
command -v npm >/dev/null 2>&1 || fail "npm is required. Install Node.js ${MIN_NODE_MAJOR}+ first: https://nodejs.org/"
command -v node >/dev/null 2>&1 || fail "node is required. Install Node.js ${MIN_NODE_MAJOR}+ first: https://nodejs.org/"
NODE_MAJOR="$(node -p 'Number(process.versions.node.split(".")[0])' 2>/dev/null || printf '0')"
case "$NODE_MAJOR" in
  ''|*[!0-9]*) fail "could not determine the installed Node.js version" ;;
esac
[ "$NODE_MAJOR" -ge "$MIN_NODE_MAJOR" ] || fail "Node.js ${MIN_NODE_MAJOR}+ is required; found $(node --version)"
pass "Node.js $(node --version) is supported (requires ${MIN_NODE_MAJOR}+)"
pass "npm $(npm --version) is available"

step "Check package availability" "npm view ${PACKAGE_NAME}@${VERSION} version"
RESOLVED_VERSION="$(npm view "${PACKAGE_NAME}@${VERSION}" version --silent 2>/dev/null)" ||
  fail "${PACKAGE_NAME}@${VERSION} is not reachable from the npm registry"
[ -n "$RESOLVED_VERSION" ] || fail "${PACKAGE_NAME}@${VERSION} returned no version from the npm registry"
pass "${PACKAGE_NAME}@${RESOLVED_VERSION} is available"

step "Choose installation target" "npm config get prefix"
if [ -z "$PREFIX" ]; then
  NPM_PREFIX="$(npm config get prefix)"
  if [ -d "$NPM_PREFIX" ] && [ -w "$NPM_PREFIX" ]; then
    PREFIX="$NPM_PREFIX"
  elif [ -d "$(dirname "$NPM_PREFIX")" ] && [ -w "$(dirname "$NPM_PREFIX")" ]; then
    PREFIX="$NPM_PREFIX"
  else
    [ -n "${HOME:-}" ] || fail "HOME is not set; pass --prefix PATH"
    PREFIX="$DEFAULT_USER_PREFIX"
  fi
fi

BIN_DIR="$PREFIX/bin"
pass "Using $PREFIX"

add_to_path() {
  case ":${PATH:-}:" in
    *":$BIN_DIR:"*) return ;;
  esac

  if [ "$MODIFY_PATH" -eq 0 ]; then
    say "Add this to your shell profile:"
    say "  export PATH=\"$BIN_DIR:\$PATH\""
    return
  fi

  SHELL_NAME="$(basename "${SHELL:-sh}")"
  case "$SHELL_NAME" in
    zsh) PROFILE="${ZDOTDIR:-$HOME}/.zshrc" ;;
    bash) PROFILE="$HOME/.bashrc" ;;
    *) PROFILE="$HOME/.profile" ;;
  esac

  PATH_LINE="export PATH=\"$BIN_DIR:\$PATH\""
  if [ -f "$PROFILE" ] && grep -F "$PATH_LINE" "$PROFILE" >/dev/null 2>&1; then
    return
  fi
  printf '\n# Added by the kmc installer\n%s\n' "$PATH_LINE" >>"$PROFILE"
  say "Updated $PROFILE"
}

case "$COMMAND" in
  install|update)
    step "Install package" "npm install --global --no-progress --prefix \"$PREFIX\" ${PACKAGE_NAME}@${VERSION}"
    mkdir -p "$PREFIX"
    npm install --global --no-progress --prefix "$PREFIX" "${PACKAGE_NAME}@${VERSION}"
    [ -x "$BIN_DIR/kmc" ] || fail "npm completed, but $BIN_DIR/kmc was not created"
    add_to_path
    pass "Package installed"
    step "Verify installation" "\"$BIN_DIR/kmc\" --version"
    INSTALLED_VERSION="$("$BIN_DIR/kmc" --version)"
    pass "kmc $INSTALLED_VERSION is ready"
    say ""
    say "${GREEN}${BOLD}Installation complete.${RESET} Run: kmc"
    ;;
  uninstall)
    step "Remove package" "npm uninstall --global --prefix \"$PREFIX\" $PACKAGE_NAME"
    say "Removing $PACKAGE_NAME from $PREFIX ..."
    npm uninstall --global --prefix "$PREFIX" "$PACKAGE_NAME"
    say "kmc was removed from $PREFIX"
    ;;
  doctor)
    step "Inspect installation" "\"$BIN_DIR/kmc\" --version"
    say "Node.js: $(node --version)"
    say "npm: $(npm --version)"
    say "Prefix: $PREFIX"
    if [ -x "$BIN_DIR/kmc" ]; then
      say "kmc: $("$BIN_DIR/kmc" --version) ($BIN_DIR/kmc)"
    elif command -v kmc >/dev/null 2>&1; then
      say "kmc: $(kmc --version) ($(command -v kmc))"
    else
      fail "kmc is not installed"
    fi
    case ":${PATH:-}:" in
      *":$BIN_DIR:"*) say "PATH: ready" ;;
      *) say "PATH: $BIN_DIR is missing" ;;
    esac
    ;;
esac
