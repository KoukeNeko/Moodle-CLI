# Installation

[繁體中文](Installation-zh-TW) · [Home](Home)

Check the [Releases page](https://github.com/KoukeNeko/Moodle-CLI/releases) before using a package
manager: these commands work after the first stable release and package entries are public. If
there is no published release yet, [build from source](#build-from-source).

## Homebrew (macOS or Linux)

Use the project's tap; this formula is not in Homebrew Core:

```sh
brew tap KoukeNeko/tap
brew install koukeneko/tap/moodle-cli
moodle version
```

To upgrade later, run `brew update` followed by `brew upgrade moodle-cli`. The formula contains
release-specific SHA-256 hashes for macOS and Linux, on amd64 and arm64.

## Scoop (Windows)

Add the project's bucket and install its manifest:

```powershell
scoop bucket add koukeneko https://github.com/KoukeNeko/scoop-bucket
scoop install koukeneko/moodle-cli
moodle version
```

To upgrade later, run `scoop update` followed by `scoop update moodle-cli`. The manifest supports
Windows amd64 and arm64, with a separate SHA-256 hash for each archive. Windows binaries are not
Authenticode-signed, so SmartScreen may ask for confirmation.

## Direct download

The [Releases page](https://github.com/KoukeNeko/Moodle-CLI/releases) lists the published
archives, `checksums.txt`, and its keyless workflow signature. Select the archive for your OS and
architecture, compare its SHA-256 digest against `checksums.txt`, then put the extracted `moodle`
(`moodle.exe` on Windows) on your `PATH`. Do not run an asset whose checksum does not match.

Package repositories update only after the stable release passes real-macOS signing and
notarization verification. If the newest tag is not yet in the tap or bucket, use the previous
stable package or wait for the release workflow to complete.

## Build from source

Use Git, GNU Make, and Go 1.26 or newer:

```sh
git clone https://github.com/KoukeNeko/Moodle-CLI.git
cd Moodle-CLI
make build
./bin/moodle version
```

Install to `~/.local/bin`:

```sh
make install
```

Override `PREFIX` or `BINDIR` when another destination is needed:

```sh
make install PREFIX=/opt/moodle-cli
make install BINDIR="$HOME/bin"
```

`make build` produces a pure-Go binary at `bin/moodle`.

## Verify the build

```sh
make verify
```

This runs unit and contract tests, the race detector, formatting and vet checks, then builds the
binary. Docker-backed Moodle tests are separate because they download images and create local test
sites; see [Development and testing](Development-and-Testing).

## Platform note

The CLI and manual login flow build on Linux, macOS, and Windows. The automatic browser callback
handler is currently Linux-only because it uses a per-user D-Bus service. Other platforms should use
`moodle auth login --method manual`.
