#!/usr/bin/env sh
# Render the Homebrew formula for one already-published stable release.
set -eu

repository_url="https://github.com/KoukeNeko/Moodle-CLI"
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
. "$script_dir/release-artifacts.sh"

release_inputs "$@"
tag=$RELEASE_TAG
version=$RELEASE_VERSION

darwin_arm64=$(release_digest "moodle-cli_${version}_darwin_arm64.tar.gz")
darwin_amd64=$(release_digest "moodle-cli_${version}_darwin_amd64.tar.gz")
linux_arm64=$(release_digest "moodle-cli_${version}_linux_arm64.tar.gz")
linux_amd64=$(release_digest "moodle-cli_${version}_linux_amd64.tar.gz")

cat <<FORMULA
class MoodleCli < Formula
  desc "Independent Moodle command-line client for learners and educators"
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
