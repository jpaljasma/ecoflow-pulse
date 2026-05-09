#!/bin/sh
set -eu

root_dir="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
script="$root_dir/scripts/local-rollout-status.sh"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

kubectl_stub="$tmp_dir/kubectl-stub"
call_log="$tmp_dir/kubectl-calls.log"

cat >"$kubectl_stub" <<'STUB'
#!/bin/sh
set -eu

printf '%s\n' "$*" >> "$KUBECTL_CALL_LOG"

if [ "${LOCAL_ROLLOUT_TEST_CASE:-}" = "strict-success" ] &&
   [ "${5:-}" = "rollout" ] &&
   [ "${6:-}" = "status" ]; then
  exit 0
fi

if [ "${5:-}" = "rollout" ] &&
   [ "${6:-}" = "status" ]; then
  echo "rollout still waiting for old replicas to terminate" >&2
  exit 1
fi

if [ "${5:-}" = "get" ] &&
   [ "${6:-}" = "deploy/app" ] &&
   [ "${7:-}" = "-o" ]; then
  case "${8:-}" in
    'jsonpath={.metadata.generation}{"|"}{.status.observedGeneration}{"|"}{.spec.replicas}{"|"}{.status.updatedReplicas}{"|"}{.status.readyReplicas}{"|"}{.status.availableReplicas}{"|"}{.status.unavailableReplicas}{"|"}{.status.replicas}')
      echo "7|7|3|3|3|3|0|4"
      exit 0
      ;;
    wide)
      echo "deployment app wide"
      exit 0
      ;;
    *)
      echo "unexpected deploy output request: ${8:-}" >&2
      exit 3
      ;;
  esac
fi

if [ "${5:-}" = "describe" ]; then
  echo "describe ${6:-}"
  exit 0
fi

if [ "${5:-}" = "get" ] &&
   [ "${6:-}" = "deploy/app" ]; then
  echo "deployment app"
  exit 0
fi

echo "unexpected kubectl call: $*" >&2
exit 3
STUB
chmod +x "$kubectl_stub"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

test_available_mode_returns_when_new_replicas_are_serving() {
  : >"$call_log"
  output="$(
    KUBECTL="$kubectl_stub" \
    KUBECTL_CALL_LOG="$call_log" \
    LOCAL_ROLLOUT_WAIT_MODE=available \
    LOCAL_ROLLOUT_TEST_CASE=available-success \
    sh "$script" ctx ns app 1s 2>&1
  )" || fail "available mode should succeed once new replicas are ready and available; output: $output"

  if grep -q 'rollout status' "$call_log"; then
    fail "available mode should not wait for kubectl rollout status termination tail"
  fi

  case "$output" in
    *"available"*) ;;
    *) fail "available mode should report the availability gate; output: $output" ;;
  esac
}

test_strict_mode_preserves_kubectl_rollout_status() {
  : >"$call_log"
  output="$(
    KUBECTL="$kubectl_stub" \
    KUBECTL_CALL_LOG="$call_log" \
    LOCAL_ROLLOUT_WAIT_MODE=strict \
    LOCAL_ROLLOUT_TEST_CASE=strict-success \
    sh "$script" ctx ns app 1s 2>&1
  )" || fail "strict mode should preserve successful kubectl rollout status; output: $output"

  if ! grep -q 'rollout status deploy/app' "$call_log"; then
    fail "strict mode should call kubectl rollout status"
  fi
}

test_available_mode_returns_when_new_replicas_are_serving
test_strict_mode_preserves_kubectl_rollout_status

echo "local-rollout-status tests passed"
