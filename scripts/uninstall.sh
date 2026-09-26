#!/usr/bin/env sh
# Remove Moodle CLI from macOS or Linux.
#
# The binary goes by default. Configuration and stored credentials stay unless
# --purge is given, because uninstalling is often one step of an upgrade and
# silently discarding a login would be a poor trade to make on someone's behalf.
#
#   curl -fsSL https://raw.githubusercontent.com/KoukeNeko/Moodle-CLI/main/scripts/uninstall.sh | sh
#
# Options:
#   --purge      also remove configuration, completions and stored credentials
#   --dry-run    report what would be removed and change nothing
#
# Environment:
#   MOODLE_CLI_INSTALL_DIR  directory to remove the binary from (default: $HOME/.local/bin)
set -eu

BINARY=moodle
# The keychain service the tool stores under, which is the project name rather
# than the command name.
KEYRING_SERVICE=moodle-cli
CONFIG_DIRECTORY_NAME=moodle-cli

install_dir=${MOODLE_CLI_INSTALL_DIR:-$HOME/.local/bin}
purge=no
dry_run=no

log() { printf '%s\n' "$*"; }
warn() { printf 'uninstall: %s\n' "$*" >&2; }
fail() {
    warn "$*"
    exit 1
}

for argument in "$@"; do
    case "$argument" in
        --purge) purge=yes ;;
        --dry-run) dry_run=yes ;;
        -h | --help)
            sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *) fail "unknown option $argument" ;;
    esac
done

remove() {
    target=$1
    [ -e "$target" ] || return 0
    if [ "$dry_run" = yes ]; then
        log "would remove $target"
        return 0
    fi
    rm -rf "$target"
    log "removed $target"
}

# Deleting files Homebrew owns would leave its metadata claiming moodle-cli is
# still installed, so hand the user back to brew rather than corrupting it.
check_homebrew() {
    command -v brew >/dev/null 2>&1 || return 0
    installed=$(brew list --formula 2>/dev/null | grep -x "$KEYRING_SERVICE" || true)
    [ -n "$installed" ] || return 0
    log "moodle-cli was installed with Homebrew. Remove it with:"
    log ""
    log "    brew uninstall moodle-cli"
    log ""
    log "Then re-run this script with --purge to drop configuration and credentials."
    [ "$purge" = yes ] || exit 0
}

config_directory() {
    case "$(uname -s)" in
        Darwin) printf '%s' "$HOME/Library/Application Support" ;;
        *) printf '%s' "${XDG_CONFIG_HOME:-$HOME/.config}" ;;
    esac
}

# Credentials are in the OS keychain by default, which needs the platform's own
# tool; a store chosen with --credential-store file sits in the configuration
# directory and goes with it. What could not be reached is reported rather than
# implied to be gone.
remove_credentials() {
    if [ "$dry_run" = yes ]; then
        log "would remove keychain entries for service $KEYRING_SERVICE"
        return 0
    fi
    case "$(uname -s)" in
        Darwin)
            while security delete-generic-password -s "$KEYRING_SERVICE" >/dev/null 2>&1; do
                log "removed a keychain entry for $KEYRING_SERVICE"
            done
            ;;
        *)
            if command -v secret-tool >/dev/null 2>&1; then
                secret-tool clear service "$KEYRING_SERVICE" >/dev/null 2>&1 || true
                log "cleared libsecret entries for $KEYRING_SERVICE"
            else
                warn "secret-tool is not installed, so anything in the keychain was left there"
            fi
            ;;
    esac
}

# Completions are removed from the same directories the installer writes to,
# and only when the file is one this tool wrote, so a hand-made completion of
# the same name is never destroyed.
remove_completions() {
    zsh_dirs="${HOMEBREW_PREFIX:-/opt/homebrew}/share/zsh/site-functions /usr/local/share/zsh/site-functions $HOME/.zsh/completions"
    bash_dirs="${HOMEBREW_PREFIX:-/opt/homebrew}/etc/bash_completion.d $HOME/.local/share/bash-completion/completions /usr/local/etc/bash_completion.d"
    fish_dirs="${XDG_CONFIG_HOME:-$HOME/.config}/fish/completions"
    for directory in $zsh_dirs; do remove_completion_file "$directory/_$BINARY"; done
    for directory in $bash_dirs; do remove_completion_file "$directory/$BINARY"; done
    for directory in $fish_dirs; do remove_completion_file "$directory/$BINARY.fish"; done
}

remove_completion_file() {
    target=$1
    [ -f "$target" ] || return 0
    # A generated completion names the command it completes; anything else is
    # somebody's own file and is left alone.
    grep -q "$BINARY" "$target" 2>/dev/null || return 0
    remove "$target"
}

main() {
    check_homebrew

    remove "$install_dir/$BINARY"
    remove_completions

    if [ "$purge" = yes ]; then
        # The configuration directory holds config.yaml and, for anyone who
        # chose it, credentials.json.
        remove "$(config_directory)/$CONFIG_DIRECTORY_NAME"
        remove_credentials
        log ""
        log "Tokens are not revoked on the Moodle side: the same token is often the"
        log "one your phone's Moodle app holds. To revoke it, use the site's own"
        log "\"Security keys\" page in your profile."
    else
        log ""
        log "Configuration and credentials were kept. Pass --purge to remove them too."
    fi
}

main
