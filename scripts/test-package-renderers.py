#!/usr/bin/env python3
"""Offline contracts for the Homebrew and Scoop release manifests."""
from __future__ import annotations

import json
import pathlib
import subprocess
import tempfile


ROOT = pathlib.Path(__file__).resolve().parents[1]
FORMULA = ROOT / "scripts/render-homebrew-formula.sh"
MANIFEST = ROOT / "scripts/render-scoop-manifest.sh"
VERSION = "0.1.0"
TAG = "v" + VERSION
ARCHIVES = (
    "darwin_arm64.tar.gz", "darwin_amd64.tar.gz",
    "linux_arm64.tar.gz", "linux_amd64.tar.gz",
    "windows_amd64.zip", "windows_arm64.zip",
)


def render(script: pathlib.Path, tag: str, checksums: pathlib.Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(["sh", str(script), tag, str(checksums)], text=True,
                          capture_output=True, check=False)


def main() -> int:
    with tempfile.TemporaryDirectory(prefix="moodle-package-renderers-") as directory:
        checksums = pathlib.Path(directory) / "checksums.txt"
        digests = {suffix: format(index, "064x") for index, suffix in enumerate(ARCHIVES, 1)}
        checksums.write_text("".join(
            f"{digest}  moodle-cli_{VERSION}_{suffix}\n"
            for suffix, digest in digests.items()))

        formula = render(FORMULA, TAG, checksums)
        assert formula.returncode == 0, formula.stderr
        assert 'desc "Independent Moodle command-line client for learners and educators"' in formula.stdout
        assert 'version "0.1.0"' in formula.stdout
        for suffix in ARCHIVES[:4]:
            assert f"moodle-cli_{VERSION}_{suffix}" in formula.stdout
            assert f'sha256 "{digests[suffix]}"' in formula.stdout

        scoop = render(MANIFEST, TAG, checksums)
        assert scoop.returncode == 0, scoop.stderr
        document = json.loads(scoop.stdout)
        assert document["version"] == VERSION
        assert document["bin"] == "moodle.exe"
        assert set(document["architecture"]) == {"64bit", "arm64"}
        for arch, suffix in (("64bit", "windows_amd64.zip"),
                             ("arm64", "windows_arm64.zip")):
            row = document["architecture"][arch]
            assert row["url"].endswith(f"/{TAG}/moodle-cli_{VERSION}_{suffix}")
            assert row["hash"] == digests[suffix]

        for bad_tag in ("v0.1.0-rc1", "v0.1.0\"", "v01.0.0", "v0.1", "main"):
            for script in (FORMULA, MANIFEST):
                result = render(script, bad_tag, checksums)
                assert result.returncode != 0 and not result.stdout, (script, bad_tag)

        checksums.write_text(checksums.read_text() +
                             f"{digests['windows_amd64.zip']}  moodle-cli_{VERSION}_windows_amd64.zip\n")
        result = render(MANIFEST, TAG, checksums)
        assert result.returncode != 0 and not result.stdout
        checksums.write_text("".join(
            f"{digest}  moodle-cli_{VERSION}_{suffix}\n"
            for suffix, digest in digests.items() if suffix != "linux_arm64.tar.gz"))
        result = render(FORMULA, TAG, checksums)
        assert result.returncode != 0 and not result.stdout

    print("Homebrew and Scoop release renderers passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
