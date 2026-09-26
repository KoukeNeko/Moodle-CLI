#!/usr/bin/env sh
# Install Moodle CLI on macOS or Linux.
#
# The archive is never trusted on its own: this downloads the release's
# checksums.txt alongside it and refuses to install anything whose digest does
# not match, so an interrupted or tampered download fails loudly instead of
# leaving a broken binary on PATH.
#
#   curl -fsSL https://raw.githubusercontent.com/KoukeNeko/Moodle-CLI/main/scripts/install.sh | sh
#
# Environment:
#   MOODLE_CLI_VERSION      release tag to install, for example v0.1.4 (default: latest)
#   MOODLE_CLI_INSTALL_DIR  directory to install into (default: $HOME/.local/bin)
set -eu

REPOSITORY="KoukeNeko/Moodle-CLI"
LATEST_RELEASE_URL="https://github.com/$REPOSITORY/releases/latest"
DOWNLOAD_BASE="https://github.com/$REPOSITORY/releases/download"

version=${MOODLE_CLI_VERSION:-}
install_dir=${MOODLE_CLI_INSTALL_DIR:-$HOME/.local/bin}

log() { printf '%s\n' "$*"; }
fail() {
    printf 'install: %s\n' "$*" >&2
    exit 1
}

require() {
    command -v "$1" >/dev/null 2>&1 || fail "$1 is required but was not found"
}

detect_platform() {
    kernel=$(uname -s)
    case "$kernel" in
        Darwin) os=darwin ;;
        Linux) os=linux ;;
        *) fail "unsupported operating system $kernel; on Windows run scripts/install.ps1" ;;
    esac
    machine=$(uname -m)
    case "$machine" in
        x86_64 | amd64) arch=amd64 ;;
        arm64 | aarch64) arch=arm64 ;;
        *) fail "unsupported architecture $machine" ;;
    esac
}

resolve_version() {
    [ -n "$version" ] && return 0
    # github.com redirects the latest release to its tag URL. Resolving the tag
    # that way avoids api.github.com, whose anonymous rate limit is shared by
    # IP and is routinely exhausted on CI runners and behind a campus NAT.
    # The latest release is never a pre-release, so a release candidate is only
    # installed when asked for by name.
    effective=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$LATEST_RELEASE_URL") ||
        fail "could not reach GitHub to find the latest release; set MOODLE_CLI_VERSION"
    version=${effective##*/}
    case "$version" in
        v[0-9]*) ;;
        *) fail "could not read a release tag from $effective; set MOODLE_CLI_VERSION" ;;
    esac
}

verify_checksum() {
    archive=$1
    sums=$2
    # goreleaser writes "<digest>  <filename>"; the file lists every archive of
    # the release, so the name has to be matched rather than the first line read.
    expected=$(awk -v name="$archive" '$2 == name { print $1 }' "$sums")
    [ -n "$expected" ] || fail "$archive is not listed in checksums.txt"
    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$archive" | awk '{print $1}')
    else
        actual=$(shasum -a 256 "$archive" | awk '{print $1}')
    fi
    [ "$expected" = "$actual" ] || fail "checksum mismatch for $archive: expected $expected, got $actual"
    log "Verified SHA-256 of $archive"
}

main() {
    require curl
    require tar
    require awk
    command -v sha256sum >/dev/null 2>&1 || require shasum

    detect_platform
    resolve_version
    numeric_version=${version#v}
    archive="moodle-cli_${numeric_version}_${os}_${arch}.tar.gz"

    work_dir=$(mktemp -d)
    trap 'rm -rf "$work_dir"' EXIT INT TERM

    log "Downloading $archive ($version)"
    curl -fsSL -o "$work_dir/$archive" "$DOWNLOAD_BASE/$version/$archive" ||
        fail "could not download $archive; check that $version exists for $os/$arch"
    curl -fsSL -o "$work_dir/checksums.txt" "$DOWNLOAD_BASE/$version/checksums.txt" ||
        fail "could not download checksums.txt for $version"

    (cd "$work_dir" && verify_checksum "$archive" checksums.txt)
    tar -xzf "$work_dir/$archive" -C "$work_dir"

    # The archive holds the binary at its root, beside README.md and LICENSE.
    extracted="$work_dir/moodle"
    [ -f "$extracted" ] || fail "the archive did not contain a moodle binary"

    mkdir -p "$install_dir"
    # Installed under a temporary name first, so a moodle that is running is
    # never replaced halfway through.
    cp "$extracted" "$install_dir/.moodle.new"
    chmod 0755 "$install_dir/.moodle.new"
    mv "$install_dir/.moodle.new" "$install_dir/moodle"

    log "Installed $("$install_dir/moodle" version | head -1) to $install_dir/moodle"

    case ":$PATH:" in
        *":$install_dir:"*) ;;
        *) log "Add $install_dir to PATH to run moodle from anywhere." ;;
    esac

    install_completions
}

# Completions go only into directories that already exist and are writable.
# Creating them would litter the home directory of someone who does not use
# that shell, and guessing at a private layout would write a file nobody finds.
completion_directory() {
    case "$1" in
        zsh)
            for candidate in "${HOMEBREW_PREFIX:-/opt/homebrew}/share/zsh/site-functions" \
                "/usr/local/share/zsh/site-functions" "$HOME/.zsh/completions"; do
                [ -d "$candidate" ] && [ -w "$candidate" ] && { printf '%s' "$candidate"; return 0; }
            done
            ;;
        bash)
            for candidate in "${HOMEBREW_PREFIX:-/opt/homebrew}/etc/bash_completion.d" \
                "$HOME/.local/share/bash-completion/completions" "/usr/local/etc/bash_completion.d"; do
                [ -d "$candidate" ] && [ -w "$candidate" ] && { printf '%s' "$candidate"; return 0; }
            done
            ;;
        fish)
            candidate="${XDG_CONFIG_HOME:-$HOME/.config}/fish/completions"
            [ -d "$candidate" ] && [ -w "$candidate" ] && { printf '%s' "$candidate"; return 0; }
            ;;
    esac
    return 1
}

completion_filename() {
    case "$1" in
        zsh) printf '_moodle' ;;
        bash) printf 'moodle' ;;
        fish) printf 'moodle.fish' ;;
    esac
}

install_completions() {
    installed=no
    for shell in zsh bash fish; do
        directory=$(completion_directory "$shell") || continue
        target="$directory/$(completion_filename "$shell")"
        # Not "completion": that name belongs to Moodle's own activity
        # completion, so the generator is called shell-completion.
        if "$install_dir/moodle" shell-completion "$shell" > "$target" 2>/dev/null; then
            log "Installed $shell completion to $target"
            installed=yes
        else
            rm -f "$target"
        fi
    done
    [ "$installed" = yes ] && return 0
    log "Shell completion: moodle shell-completion bash|zsh|fish|powershell"
}

# Sourcing with MOODLE_CLI_INSTALL_LIB=1 exposes the functions without
# installing anything, so the checksum guard can be tested directly rather
# than trusted.
[ "${MOODLE_CLI_INSTALL_LIB:-}" = "1" ] || main "$@"
