#!/usr/bin/env sh
# Render the Homebrew formula for one already-published stable release.
set -eu

repository_url="https://github.com/KoukeNeko/Moodle-CLI"

[ "$#" -eq 2 ] || {
    printf '%s\n' "usage: $0 <version-tag> <checksums-file>" >&2
    exit 2
}
tag=$1
checksums=$2

case "$tag" in
    v*) version=${tag#v} ;;
    *) printf '%s\n' "version tag $tag must start with v" >&2; exit 2 ;;
esac
case "$version" in
    *-*) printf '%s\n' "refusing to render a pre-release formula" >&2; exit 2 ;;
esac
[ -f "$checksums" ] || {
    printf '%s\n' "no such checksum file: $checksums" >&2
    exit 2
}

digest_for() {
    archive="moodle-cli_${version}_$1.tar.gz"
    digest=$(awk -v name="$archive" '$2 == name { print $1 }' "$checksums")
    [ -n "$digest" ] || {
        printf '%s\n' "no digest for $archive" >&2
        exit 1
    }
    printf '%s' "$digest"
}

darwin_arm64=$(digest_for darwin_arm64)
darwin_amd64=$(digest_for darwin_amd64)
linux_arm64=$(digest_for linux_arm64)
linux_amd64=$(digest_for linux_amd64)

cat <<FORMULA
class MoodleCli < Formula
  desc "Independent Moodle command-line client for students"
  homepage "$repository_url"
  version "$version"
  license "MIT"

  livecheck do
    url :homepage
    strategy :github_latest
  end

  on_macos do
    on_arm do
      url "$repository_url/releases/download/$tag/moodle-cli_${version}_darwin_arm64.tar.gz"
      sha256 "$darwin_arm64"
    end
    on_intel do
      url "$repository_url/releases/download/$tag/moodle-cli_${version}_darwin_amd64.tar.gz"
      sha256 "$darwin_amd64"
    end
  end

  on_linux do
    on_arm do
      url "$repository_url/releases/download/$tag/moodle-cli_${version}_linux_arm64.tar.gz"
      sha256 "$linux_arm64"
    end
    on_intel do
      url "$repository_url/releases/download/$tag/moodle-cli_${version}_linux_amd64.tar.gz"
      sha256 "$linux_amd64"
    end
  end

  def install
    bin.install "moodle"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/moodle version")
    assert_match '"schema_version":1', shell_output("#{bin}/moodle version --json")
  end
end
FORMULA
