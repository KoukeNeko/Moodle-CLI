"""Say whether two runs measured the same thing.

Exits 0 when they are comparable and 1 with a reason when they are not. The
reason is printed rather than raised so the caller can show it to a person.

A run with no run.json predates the manifest, and nothing about it is
knowable — not its Moodle version, not whether it finished. An explicitly
named pair is allowed through so an old log can still be read on purpose; one
chosen automatically is not, because the whole point of choosing is to find a
baseline that measured the same thing.
"""

import json
import os
import sys


def load(path):
    try:
        with open(os.path.join(path, "run.json"), encoding="utf-8") as handle:
            return json.load(handle)
    except OSError:
        return None


# --auto marks a baseline the caller picked rather than one a person named.
auto = "--auto" in sys.argv
new, old = load(sys.argv[1]), load(sys.argv[2])
if new is None or old is None:
    if auto:
        print("其中一輪沒有 run.json，看不出量的是不是同一件事")
        sys.exit(1)
    sys.exit(0)

if not new["complete"]:
    print("這一輪沒有跑完")
    sys.exit(1)
if not old["complete"]:
    # The one that cost the most to diagnose: a partial transcript makes every
    # command after the stopping point look as though it had disappeared.
    print("基準那一輪沒有跑完")
    sys.exit(1)
if new["moodle_release"] != old["moodle_release"]:
    print("Moodle 版本不同（%s vs %s）" % (new["moodle_release"], old["moodle_release"]))
    sys.exit(1)
if new["site"] != old["site"]:
    print("跑的站台組合不同")
    sys.exit(1)
sys.exit(0)
