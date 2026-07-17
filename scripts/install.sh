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

say() {
  if [ "$QUIET" -eq 0 ]; then
    printf '%s\n' "$*"
  fi
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

command -v npm >/dev/null 2>&1 || fail "npm is required. Install Node.js ${MIN_NODE_MAJOR}+ first: https://nodejs.org/"
command -v node >/dev/null 2>&1 || fail "node is required. Install Node.js ${MIN_NODE_MAJOR}+ first: https://nodejs.org/"

NODE_MAJOR="$(node -p 'Number(process.versions.node.split(".")[0])' 2>/dev/null || printf '0')"
case "$NODE_MAJOR" in
  ''|*[!0-9]*) fail "could not determine the installed Node.js version" ;;
esac
[ "$NODE_MAJOR" -ge "$MIN_NODE_MAJOR" ] || fail "Node.js ${MIN_NODE_MAJOR}+ is required; found $(node --version)"

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
    mkdir -p "$PREFIX"
    say "Installing ${PACKAGE_NAME}@${VERSION} into $PREFIX ..."
    npm install --global --prefix "$PREFIX" "${PACKAGE_NAME}@${VERSION}"
    [ -x "$BIN_DIR/kmc" ] || fail "npm completed, but $BIN_DIR/kmc was not created"
    add_to_path
    say "Installed: $("$BIN_DIR/kmc" --version)"
    say "Run: kmc"
    ;;
  uninstall)
    say "Removing $PACKAGE_NAME from $PREFIX ..."
    npm uninstall --global --prefix "$PREFIX" "$PACKAGE_NAME"
    say "kmc was removed from $PREFIX"
    ;;
  doctor)
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
