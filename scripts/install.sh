#!/bin/sh

set -eu

REPOSITORY="${KMC_GITHUB_REPOSITORY:-marius4lui/kmc}"
API_URL="${KMC_GITHUB_API_URL:-https://api.github.com}"
DOWNLOAD_URL="${KMC_GITHUB_DOWNLOAD_URL:-https://github.com}"
COMMAND="install"
CHANNEL="stable"
VERSION=""
PREFIX=""
MODIFY_PATH=0
QUIET=0
VERIFY_SIGNATURE=0

say() {
  [ "$QUIET" -eq 1 ] || printf '%s\n' "$*"
}

fail() {
  printf 'kmc installer: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Install and manage the native kmc binary.

Usage:
  install.sh [install|update|uninstall|doctor] [options]

Options:
  --channel CHANNEL  Release channel: stable or beta (default: stable)
  --version VERSION  Install an exact GitHub release (for example v2.0.0)
  --prefix PATH      Install below PATH (default: ~/.local)
  --modify-path      Add the selected bin directory to your shell profile
  --verify-signature Verify the GitHub artifact attestation (requires gh)
  --quiet            Only print errors
  -h, --help         Show this help

Examples:
  curl -fsSL https://kmc.kmuc.app/install.sh | sh
  curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- --channel beta
  curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- --version v2.0.0
  curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- uninstall
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    install|update|uninstall|doctor) COMMAND="$1" ;;
    --channel)
      [ "$#" -ge 2 ] || fail "--channel requires a value"
      CHANNEL="$2"
      shift
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
    --modify-path) MODIFY_PATH=1 ;;
    --verify-signature) VERIFY_SIGNATURE=1 ;;
    --quiet) QUIET=1 ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown argument: $1 (use --help)" ;;
  esac
  shift
done

case "$CHANNEL" in
  experimental) CHANNEL="beta" ;;
  stable|beta) ;;
  *) fail "unsupported channel '$CHANNEL' (expected stable or beta)" ;;
esac

[ -n "$PREFIX" ] || {
  [ -n "${HOME:-}" ] || fail "HOME is not set; pass --prefix PATH"
  PREFIX="$HOME/.local"
}
BIN_DIR="$PREFIX/bin"
BINARY="$BIN_DIR/kmc"
STATE_DIR="$PREFIX/share/kmc"
STATE_FILE="$STATE_DIR/install.env"

detect_platform() {
  case "$(uname -s 2>/dev/null)" in
    Linux) OS="linux" ;;
    Darwin) OS="darwin" ;;
    *) fail "unsupported operating system: $(uname -s 2>/dev/null || printf unknown)" ;;
  esac
  case "$(uname -m 2>/dev/null)" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) fail "unsupported architecture: $(uname -m 2>/dev/null || printf unknown)" ;;
  esac
}

require_download_tools() {
  command -v curl >/dev/null 2>&1 || fail "curl is required"
  command -v tar >/dev/null 2>&1 || fail "tar is required"
  if command -v sha256sum >/dev/null 2>&1; then
    HASH_TOOL="sha256sum"
  elif command -v shasum >/dev/null 2>&1; then
    HASH_TOOL="shasum"
  else
    fail "sha256sum or shasum is required"
  fi
}

resolve_version() {
  if [ -n "$VERSION" ]; then
    case "$VERSION" in v*) ;; *) VERSION="v$VERSION" ;; esac
    return
  fi

  if [ "$CHANNEL" = "stable" ]; then
    VERSION="$(curl -fsSL -H 'Accept: application/vnd.github+json' \
      "$API_URL/repos/$REPOSITORY/releases/latest" |
      sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
      head -n 1)"
  else
    VERSION="$(curl -fsSL -H 'Accept: application/vnd.github+json' \
      "$API_URL/repos/$REPOSITORY/releases?per_page=100" |
      tr '\n' ' ' | sed 's/"tag_name"/\
"tag_name"/g' |
      awk '/"prerelease"[[:space:]]*:[[:space:]]*true/ {
        if (match($0, /"tag_name"[[:space:]]*:[[:space:]]*"[^"]+"/)) {
          value = substr($0, RSTART, RLENGTH)
          sub(/^"tag_name"[[:space:]]*:[[:space:]]*"/, "", value)
          sub(/"$/, "", value)
          print value; exit
        }
      }')"
  fi
  [ -n "$VERSION" ] || fail "no $CHANNEL release is available"
}

add_to_path() {
  case ":${PATH:-}:" in *":$BIN_DIR:"*) return ;; esac
  if [ "$MODIFY_PATH" -eq 0 ]; then
    say "Add this to your shell profile:"
    say "  export PATH=\"$BIN_DIR:\$PATH\""
    return
  fi
  shell_name="$(basename "${SHELL:-sh}")"
  case "$shell_name" in
    zsh) profile="${ZDOTDIR:-$HOME}/.zshrc" ;;
    bash) profile="$HOME/.bashrc" ;;
    *) profile="$HOME/.profile" ;;
  esac
  path_line="export PATH=\"$BIN_DIR:\$PATH\""
  if ! { [ -f "$profile" ] && grep -F "$path_line" "$profile" >/dev/null 2>&1; }; then
    printf '\n# Added by the kmc installer\n%s\n' "$path_line" >>"$profile"
    say "Updated $profile"
  fi
}

install_binary() {
  require_download_tools
  detect_platform
  resolve_version
  plain_version="${VERSION#v}"
  asset="kmc_${plain_version}_${OS}_${ARCH}.tar.gz"
  base="$DOWNLOAD_URL/$REPOSITORY/releases/download/$VERSION"
  tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/kmc-install.XXXXXX")"
  trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

  say "Downloading kmc $VERSION ($CHANNEL) for $OS/$ARCH..."
  curl -fL --retry 3 --proto '=https' --tlsv1.2 -o "$tmp_dir/$asset" "$base/$asset"
  curl -fL --retry 3 --proto '=https' --tlsv1.2 -o "$tmp_dir/kmc_checksums.txt" "$base/kmc_checksums.txt"
  expected="$(awk -v asset="$asset" '$2 == asset || $2 == "*" asset { print $1; exit }' "$tmp_dir/kmc_checksums.txt")"
  [ -n "$expected" ] || fail "checksum for $asset is missing"
  if [ "$HASH_TOOL" = "sha256sum" ]; then
    actual="$(sha256sum "$tmp_dir/$asset" | awk '{print $1}')"
  else
    actual="$(shasum -a 256 "$tmp_dir/$asset" | awk '{print $1}')"
  fi
  [ "$actual" = "$expected" ] || fail "SHA-256 verification failed for $asset"
  if [ "$VERIFY_SIGNATURE" -eq 1 ]; then
    command -v gh >/dev/null 2>&1 || fail "gh is required for --verify-signature"
    gh attestation verify "$tmp_dir/$asset" --repo "$REPOSITORY" >/dev/null ||
      fail "GitHub artifact attestation verification failed for $asset"
  fi

  mkdir -p "$tmp_dir/unpacked"
  tar -xzf "$tmp_dir/$asset" -C "$tmp_dir/unpacked"
  [ -f "$tmp_dir/unpacked/kmc" ] || fail "release archive does not contain kmc"
  chmod 755 "$tmp_dir/unpacked/kmc"
  "$tmp_dir/unpacked/kmc" --version >/dev/null 2>&1 ||
    fail "downloaded binary did not pass its version check"

  mkdir -p "$BIN_DIR" "$STATE_DIR"
  staged="$BIN_DIR/.kmc.new.$$"
  cp "$tmp_dir/unpacked/kmc" "$staged"
  chmod 755 "$staged"
  mv -f "$staged" "$BINARY"
  "$BINARY" channel set "$CHANNEL" >/dev/null 2>&1 ||
    fail "installed binary could not save the $CHANNEL update channel"
  {
    printf 'VERSION=%s\n' "$VERSION"
    printf 'CHANNEL=%s\n' "$CHANNEL"
    printf 'OS=%s\n' "$OS"
    printf 'ARCH=%s\n' "$ARCH"
  } >"$STATE_FILE"
  add_to_path
  say "kmc $VERSION installed at $BINARY"
}

case "$COMMAND" in
  install|update) install_binary ;;
  uninstall)
    if [ -e "$BINARY" ]; then
      rm -f "$BINARY"
      say "Removed $BINARY"
    else
      say "kmc is not installed at $BINARY"
    fi
    rm -f "$STATE_FILE"
    ;;
  doctor)
    detect_platform
    say "Repository: $REPOSITORY"
    say "Platform: $OS/$ARCH"
    say "Channel: $CHANNEL"
    say "Prefix: $PREFIX"
    if [ -x "$BINARY" ]; then
      say "kmc: $("$BINARY" --version) ($BINARY)"
    elif command -v kmc >/dev/null 2>&1; then
      found="$(command -v kmc)"
      say "kmc: $(kmc --version) ($found)"
    else
      fail "kmc is not installed"
    fi
    [ ! -f "$STATE_FILE" ] || say "Installer metadata: $STATE_FILE"
    case ":${PATH:-}:" in
      *":$BIN_DIR:"*) say "PATH: ready" ;;
      *) say "PATH: $BIN_DIR is missing" ;;
    esac
    if command -v npm >/dev/null 2>&1 &&
      npm list --global --depth=0 @marius4lui/kmc >/dev/null 2>&1; then
      say "Legacy npm installation: detected (remove manually with npm uninstall -g @marius4lui/kmc)"
    else
      say "Legacy npm installation: not detected"
    fi
    ;;
esac
