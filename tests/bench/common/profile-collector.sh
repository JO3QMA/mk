#!/bin/sh
# Capture pprof profiles from the mk-go app container while k6 is running
# the per-scenario load. See `tests/bench/docker-compose.bench.yml` for how
# this is wired into the bench profile.
#
# Output layout (under $OUT, default /output/profiles):
#   heap-pre.pb.gz / heap-post.pb.gz       — heap snapshots before & after
#   allocs-post.pb.gz                       — total alloc samples after run
#   goroutine-pre.pb.gz / goroutine-post.pb.gz
#   cpu-<scenario>.pb.gz                    — CPU profile for each k6 scenario
#
# Misskey TS 側はここでは取らない (Node.js は別ツール、本ロードマップ #413 は
# Go 側のみが対象)。
set -eu

HOST=${HOST:-app-mkgo:3000}
OUT=${OUT:-/output/profiles}
SCENARIO_DURATION=${SCENARIO_DURATION:-31}
PROFILE_SECONDS=${PROFILE_SECONDS:-25}
SCENARIOS=${SCENARIOS:-"ping meta local-timeline users-show i notes-create"}

mkdir -p "$OUT"
rm -f "$OUT"/*.pb.gz 2>/dev/null || true

ts() { date -Is; }
log() { echo "[$(ts)] profile-collector: $*"; }

snap() {
  endpoint=$1
  outfile=$2
  if curl -fsS --max-time 30 "http://${HOST}/debug/pprof/${endpoint}" -o "$outfile"; then
    log "saved $(basename "$outfile") ($(wc -c < "$outfile") bytes)"
  else
    log "WARN: failed to fetch /debug/pprof/${endpoint}"
  fi
}

# k6 が ramp-up を始めるまで少し待つ。k6-mkgo と本コンテナは
# seed-mkgo が success した直後に同時起動するので、3秒で十分。
log "waiting 3s for k6 ramp-up (host=${HOST})"
sleep 3

snap heap "$OUT/heap-pre.pb.gz"
snap goroutine "$OUT/goroutine-pre.pb.gz"

i=0
for s in $SCENARIOS; do
  i=$((i + 1))
  log "scenario ${i}/6 (${s}): capturing CPU profile for ${PROFILE_SECONDS}s"
  if curl -fsS --max-time $((PROFILE_SECONDS + 10)) \
    "http://${HOST}/debug/pprof/profile?seconds=${PROFILE_SECONDS}" \
    -o "$OUT/cpu-${s}.pb.gz"; then
    log "saved cpu-${s}.pb.gz ($(wc -c < "$OUT/cpu-${s}.pb.gz") bytes)"
  else
    log "WARN: CPU profile failed for ${s}"
  fi
  # Sleep through the remainder of the scenario block (ramp-down + buffer)
  # so that the next CPU profile aligns with the next scenario's steady state.
  remaining=$((SCENARIO_DURATION - PROFILE_SECONDS - 1))
  if [ "$remaining" -gt 0 ] && [ "$i" -lt 6 ]; then
    sleep "$remaining"
  fi
done

snap heap "$OUT/heap-post.pb.gz"
snap allocs "$OUT/allocs-post.pb.gz"
snap goroutine "$OUT/goroutine-post.pb.gz"

log "done. profiles in $OUT:"
ls -la "$OUT"
