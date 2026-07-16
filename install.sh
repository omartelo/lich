#!/bin/sh
# lich installer — detects the distro, downloads the matching package from the
# latest GitHub release, verifies its checksum and installs it through the
# native package manager (which also resolves the runtime dependencies).
#
#   curl -fsSL https://raw.githubusercontent.com/omartelo/lich/main/install.sh | sh
#
# POSIX sh, no bashisms; everything runs inside main() so a truncated
# download executes nothing.

set -eu

REPO="omartelo/lich"
API="https://api.github.com/repos/${REPO}/releases/latest"

info() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

# detect_family prints deb, rpm or arch by reading /etc/os-release
# (ID first, then ID_LIKE so derivatives map to their parent).
detect_family() {
  [ -r /etc/os-release ] || fail "cannot read /etc/os-release — unsupported system"
  # shellcheck disable=SC1091
  . /etc/os-release
  for id in ${ID:-} ${ID_LIKE:-}; do
    case "$id" in
      debian | ubuntu) echo deb; return ;;
      fedora | rhel | centos) echo rpm; return ;;
      arch) echo arch; return ;;
    esac
  done
  fail "unsupported distro '${ID:-unknown}' — grab a package or the static binary from https://github.com/${REPO}/releases"
}

# latest_tag prints the tag name of the latest release, without jq. The JSON
# is captured before parsing so grep closing the pipe never trips curl.
latest_tag() {
  json="$(curl -fsSL "$API")" || return 1
  printf '%s\n' "$json" | grep -m1 '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/'
}

# have reports whether a command exists on PATH.
have() { command -v "$1" >/dev/null 2>&1; }

# as_root runs a command with sudo unless already root.
as_root() {
  if [ "$(id -u)" -eq 0 ]; then "$@"; else sudo "$@"; fi
}

check_runtime_deps() {
  browser=""
  for candidate in chromium chromium-browser google-chrome-stable google-chrome helium-browser brave; do
    if have "$candidate"; then browser="$candidate"; break; fi
  done
  missing=""
  [ -n "$browser" ] || missing="a Chromium-family browser (chromium, google-chrome, helium-browser, brave, ...)"
  if ! have zenity; then
    [ -z "$missing" ] || missing="${missing} and "
    missing="${missing}zenity"
  fi
  if [ -n "$missing" ]; then
    info "still missing: ${missing}"
    info "install through your package manager, e.g.: $1"
  else
    info "runtime dependencies present (${browser}, zenity)"
  fi
}

main() {
  [ "$(uname -s)" = "Linux" ] || fail "lich is Linux-only"
  [ "$(uname -m)" = "x86_64" ] || fail "releases ship x86_64 only — build from source: https://github.com/${REPO}"
  have curl || fail "curl is required"
  have sha256sum || fail "sha256sum is required"

  family="$(detect_family)"
  tag="$(latest_tag)"
  [ -n "$tag" ] || fail "could not resolve the latest release from ${API}"

  case "$family" in
    deb) asset="lich-${tag}-amd64.deb" ;;
    rpm) asset="lich-${tag}-x86_64.rpm" ;;
    arch) asset="lich-${tag}-x86_64.pkg.tar.zst" ;;
  esac
  base="https://github.com/${REPO}/releases/download/${tag}"

  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT

  info "downloading ${asset} (${tag})"
  curl -fsSL -o "${tmp}/${asset}" "${base}/${asset}"
  curl -fsSL -o "${tmp}/checksums.txt" "${base}/checksums.txt"

  info "verifying checksum"
  (cd "$tmp" && grep " ${asset}\$" checksums.txt | sha256sum -c -) \
    || fail "checksum mismatch for ${asset} — aborting"

  info "installing ${asset}"
  case "$family" in
    # apt/dnf resolve the Recommends (chromium, zenity) on their own;
    # pacman has no Recommends, so arch relies on the check below.
    deb) as_root apt-get install -y "${tmp}/${asset}" ;;
    rpm) as_root dnf install -y "${tmp}/${asset}" ;;
    arch) as_root pacman -U --noconfirm "${tmp}/${asset}" ;;
  esac

  case "$family" in
    deb) check_runtime_deps "sudo apt-get install chromium zenity" ;;
    rpm) check_runtime_deps "sudo dnf install chromium zenity" ;;
    arch) check_runtime_deps "sudo pacman -S chromium zenity" ;;
  esac

  info "done — run 'lich'"
}

main "$@"
