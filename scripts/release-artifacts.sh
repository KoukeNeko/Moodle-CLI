#!/usr/bin/env sh
# Shared, strict inputs for the Homebrew and Scoop release renderers.

release_inputs() {
    [ "$#" -eq 2 ] || {
        printf '%s\n' 'usage: renderer <stable-version-tag> <checksums-file>' >&2
        return 2
    }
    RELEASE_TAG=$1
    RELEASE_CHECKSUMS=$2
    if ! printf '%s\n' "$RELEASE_TAG" | LC_ALL=C grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'; then
        printf '%s\n' "refusing non-stable semantic version tag: $RELEASE_TAG" >&2
        return 2
    fi
    RELEASE_VERSION=${RELEASE_TAG#v}
    [ -f "$RELEASE_CHECKSUMS" ] || {
        printf '%s\n' "no such checksum file: $RELEASE_CHECKSUMS" >&2
        return 2
    }
}

release_digest() {
    archive=$1
    digest=$(awk -v name="$archive" '$2 == name { count++; result = $1 } END { if (count == 1) print result }' "$RELEASE_CHECKSUMS")
    if ! printf '%s\n' "$digest" | LC_ALL=C grep -Eq '^[0-9a-f]{64}$'; then
        printf '%s\n' "missing, duplicate, or invalid SHA-256 for $archive" >&2
        return 1
    fi
    printf '%s' "$digest"
}
