#!/usr/bin/env sh
# Render a Scoop manifest from the checksums of one published stable release.
set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
. "$script_dir/release-artifacts.sh"
release_inputs "$@"

version=$RELEASE_VERSION
tag=$RELEASE_TAG
amd64=$(release_digest "moodle-cli_${version}_windows_amd64.zip")
arm64=$(release_digest "moodle-cli_${version}_windows_arm64.zip")

cat <<MANIFEST
{
  "version": "$version",
  "description": "Independent Moodle command-line client for learners, educators, and administrators",
  "homepage": "https://github.com/KoukeNeko/Moodle-CLI",
  "license": "MIT",
  "architecture": {
    "64bit": {
      "url": "https://github.com/KoukeNeko/Moodle-CLI/releases/download/$tag/moodle-cli_${version}_windows_amd64.zip",
      "hash": "$amd64"
    },
    "arm64": {
      "url": "https://github.com/KoukeNeko/Moodle-CLI/releases/download/$tag/moodle-cli_${version}_windows_arm64.zip",
      "hash": "$arm64"
    }
  },
  "bin": "moodle.exe",
  "checkver": {
    "github": "https://github.com/KoukeNeko/Moodle-CLI"
  }
}
MANIFEST
