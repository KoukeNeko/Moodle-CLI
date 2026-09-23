#!/usr/bin/env bash
# Guard the long-lived self-hosted runner before a multi-hour Moodle job.
# The thresholds match the smallest supported runner: 4 CPUs, 5 GiB RAM,
# 2 GiB swap and about 20 GiB normally free on a 33 GiB system disk.
set -euo pipefail

MIN_DISK_GIB="${MIN_DISK_GIB:-10}"
MIN_MEMORY_MIB="${MIN_MEMORY_MIB:-4600}"
MIN_CPUS="${MIN_CPUS:-4}"

fail() { printf 'runner preflight: %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null || fail "docker is not installed"
docker compose version >/dev/null || fail "docker compose v2 is unavailable"
docker info >/dev/null || fail "the runner cannot reach the Docker daemon"

cpus=$(getconf _NPROCESSORS_ONLN)
memory_mib=$(awk '/^MemTotal:/ { print int($2 / 1024) }' /proc/meminfo)
swap_mib=$(awk '/^SwapTotal:/ { print int($2 / 1024) }' /proc/meminfo)
disk_kib=$(df -Pk . | awk 'NR == 2 { print $4 }')
disk_gib=$((disk_kib / 1024 / 1024))

[ "$cpus" -ge "$MIN_CPUS" ] || fail "need >= ${MIN_CPUS} CPUs; found ${cpus}"
[ "$memory_mib" -ge "$MIN_MEMORY_MIB" ] || fail "need >= ${MIN_MEMORY_MIB} MiB RAM; found ${memory_mib}"
[ "$disk_gib" -ge "$MIN_DISK_GIB" ] || fail "need >= ${MIN_DISK_GIB} GiB free; found ${disk_gib} GiB"

cat <<EOF
runner preflight passed
  CPUs:       $cpus
  RAM:        ${memory_mib} MiB
  swap:       ${swap_mib} MiB
  disk free:  ${disk_gib} GiB
  kernel:     $(uname -sr)
  Docker:     $(docker version --format '{{.Server.Version}}')
EOF
