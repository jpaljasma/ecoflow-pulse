#!/usr/bin/env bash
set -euo pipefail

root_dir="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
script="$root_dir/scripts/cloud-forward-supervisor.sh"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

make_child_stub() {
  local path="$1"
  cat >"$path" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
printf 'start\n' >> "$CLOUD_FORWARD_TEST_START_LOG"
trap 'exit 0' TERM INT
while :; do
  sleep 1
done
STUB
  chmod +x "$path"
}

test_restarts_child_when_probe_fails() {
  local probe_stub="$tmp_dir/probe-restart"
  local child_stub="$tmp_dir/child-restart"
  local probe_log="$tmp_dir/probe-restart.log"
  local start_log="$tmp_dir/start-restart.log"
  make_child_stub "$child_stub"
  cat >"$probe_stub" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
count_file="$CLOUD_FORWARD_TEST_PROBE_LOG"
count="$(wc -l < "$count_file" 2>/dev/null || echo 0)"
printf 'probe %s %s\n' "$1" "$2" >> "$count_file"
if [ "$count" -eq 0 ]; then
  exit 1
fi
exit 0
STUB
  chmod +x "$probe_stub"

  output="$(
    CLOUD_FORWARD_PROBE_BIN="$probe_stub" \
    CLOUD_FORWARD_TEST_PROBE_LOG="$probe_log" \
    CLOUD_FORWARD_TEST_START_LOG="$start_log" \
    CLOUD_FORWARD_SUPERVISOR_INTERVAL_SEC=0.1 \
    CLOUD_FORWARD_SUPERVISOR_RESTART_DELAY_SEC=0 \
    CLOUD_FORWARD_SUPERVISOR_STARTUP_GRACE_SEC=0 \
    CLOUD_FORWARD_SUPERVISOR_MAX_CYCLES=2 \
    bash "$script" test-forward postgres 25432 "$child_stub" 2>&1
  )" || fail "supervisor should recover after one failed probe; output: $output"

  starts="$(wc -l < "$start_log" | tr -d ' ')"
  if [ "$starts" -lt 2 ]; then
    fail "supervisor should restart the child after a failed probe; starts=$starts output=$output"
  fi
  case "$output" in
    *"restarting child"*) ;;
    *) fail "supervisor should report child restart; output=$output" ;;
  esac
}

test_keeps_child_when_probe_is_healthy() {
  local probe_stub="$tmp_dir/probe-healthy"
  local child_stub="$tmp_dir/child-healthy"
  local probe_log="$tmp_dir/probe-healthy.log"
  local start_log="$tmp_dir/start-healthy.log"
  make_child_stub "$child_stub"
  cat >"$probe_stub" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
printf 'probe %s %s\n' "$1" "$2" >> "$CLOUD_FORWARD_TEST_PROBE_LOG"
exit 0
STUB
  chmod +x "$probe_stub"

  output="$(
    CLOUD_FORWARD_PROBE_BIN="$probe_stub" \
    CLOUD_FORWARD_TEST_PROBE_LOG="$probe_log" \
    CLOUD_FORWARD_TEST_START_LOG="$start_log" \
    CLOUD_FORWARD_SUPERVISOR_INTERVAL_SEC=0.1 \
    CLOUD_FORWARD_SUPERVISOR_MAX_CYCLES=2 \
    bash "$script" test-forward postgres 25432 "$child_stub" 2>&1
  )" || fail "supervisor should keep running when probe is healthy; output: $output"

  starts="$(wc -l < "$start_log" | tr -d ' ')"
  if [ "$starts" -ne 1 ]; then
    fail "healthy probe should not restart the child; starts=$starts output=$output"
  fi
}

test_does_not_restart_during_startup_grace() {
  local probe_stub="$tmp_dir/probe-grace"
  local child_stub="$tmp_dir/child-grace"
  local probe_log="$tmp_dir/probe-grace.log"
  local start_log="$tmp_dir/start-grace.log"
  make_child_stub "$child_stub"
  cat >"$probe_stub" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
printf 'probe %s %s\n' "$1" "$2" >> "$CLOUD_FORWARD_TEST_PROBE_LOG"
exit 1
STUB
  chmod +x "$probe_stub"

  output="$(
    CLOUD_FORWARD_PROBE_BIN="$probe_stub" \
    CLOUD_FORWARD_TEST_PROBE_LOG="$probe_log" \
    CLOUD_FORWARD_TEST_START_LOG="$start_log" \
    CLOUD_FORWARD_SUPERVISOR_INTERVAL_SEC=0.1 \
    CLOUD_FORWARD_SUPERVISOR_STARTUP_GRACE_SEC=5 \
    CLOUD_FORWARD_SUPERVISOR_MAX_CYCLES=2 \
    bash "$script" test-forward postgres 25432 "$child_stub" 2>&1
  )" || fail "supervisor should tolerate probe failures during startup grace; output: $output"

  starts="$(wc -l < "$start_log" | tr -d ' ')"
  if [ "$starts" -ne 1 ]; then
    fail "startup grace should not restart the child; starts=$starts output=$output"
  fi
}

test_exits_on_term_signal() {
  local probe_stub="$tmp_dir/probe-term"
  local child_stub="$tmp_dir/child-term"
  local probe_log="$tmp_dir/probe-term.log"
  local start_log="$tmp_dir/start-term.log"
  make_child_stub "$child_stub"
  cat >"$probe_stub" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
printf 'probe %s %s\n' "$1" "$2" >> "$CLOUD_FORWARD_TEST_PROBE_LOG"
exit 0
STUB
  chmod +x "$probe_stub"

  CLOUD_FORWARD_PROBE_BIN="$probe_stub" \
    CLOUD_FORWARD_TEST_PROBE_LOG="$probe_log" \
    CLOUD_FORWARD_TEST_START_LOG="$start_log" \
    CLOUD_FORWARD_SUPERVISOR_INTERVAL_SEC=1 \
    bash "$script" test-forward postgres 25432 "$child_stub" >/tmp/cloud-forward-supervisor-term.out 2>&1 &
  local supervisor_pid="$!"
  sleep 0.2
  kill "$supervisor_pid"

  for _ in $(seq 1 20); do
    if ! kill -0 "$supervisor_pid" >/dev/null 2>&1; then
      wait "$supervisor_pid" >/dev/null 2>&1 || true
      return 0
    fi
    sleep 0.1
  done

  kill "$supervisor_pid" >/dev/null 2>&1 || true
  fail "supervisor should exit after TERM"
}

test_restarts_child_when_probe_fails
test_keeps_child_when_probe_is_healthy
test_does_not_restart_during_startup_grace
test_exits_on_term_signal

echo "cloud-forward-supervisor tests passed"
