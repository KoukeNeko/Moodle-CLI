"""Write one run's provenance beside its transcript.

A run is only comparable to another that measured the same thing. The fields
here are what "the same thing" means: the same Moodle release, the same build
of the CLI, the same set of sites — and, above all, a run that finished.
"""

import json
import os

path = os.environ["MANIFEST_PATH"]
with_nows = os.environ["MANIFEST_WITH_NOWS"] == "1"

manifest = {
    # False until the run reaches its own summary. A killed run leaves this
    # behind rather than leaving no evidence that it stopped early.
    "complete": os.environ["MANIFEST_COMPLETE"] == "true",
    "commands": int(os.environ["MANIFEST_COMMANDS"]),
    "cli_revision": os.environ["MANIFEST_REVISION"],
    "moodle_release": os.environ["MANIFEST_RELEASE"],
    "site": {
        "std_port": int(os.environ["MANIFEST_STD"]),
        "nows_port": int(os.environ["MANIFEST_NOWS"]) if with_nows else None,
        "tls": os.environ["MANIFEST_TLS"] == "1",
    },
}

with open(path, "w", encoding="utf-8") as out:
    json.dump(manifest, out, indent=2, sort_keys=True)
    out.write("\n")
