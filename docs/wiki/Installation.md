# Installation

[繁體中文](Installation-zh-TW) · [Home](Home)

There is no published release yet. Build from source with Git, GNU Make, and Go 1.26 or newer:

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

`make build` produces a pure-Go binary at `bin/moodle`. Release configuration exists for Linux,
macOS, and Windows on amd64 and arm64, but no release artifact should be considered available until
it appears on the repository's Releases page.

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
