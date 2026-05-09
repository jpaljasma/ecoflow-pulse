#!/bin/sh
set -eu

if [ "$#" -ne 4 ]; then
  echo "usage: $0 <context> <namespace> <deployment> <timeout>" >&2
  exit 2
fi

context="$1"
namespace="$2"
deployment="$3"
timeout="$4"
kubectl_cmd="${KUBECTL:-kubectl}"
log_tail="${ROLLOUT_DEBUG_LOG_TAIL:-160}"
wait_mode="${LOCAL_ROLLOUT_WAIT_MODE:-strict}"
poll_interval="${LOCAL_ROLLOUT_POLL_INTERVAL_SECONDS:-2}"

set -f
run_kubectl() {
  # Match Make's historical KUBECTL expansion: callers may include simple flags
  # such as KUBECTL="kubectl --request-timeout=30s".
  # shellcheck disable=SC2086
  $kubectl_cmd "$@"
}

duration_to_seconds() {
  value="$1"
  case "$value" in
    *s) value="${value%s}" ;;
    *m) value="$(( ${value%m} * 60 ))" ;;
    *h) value="$(( ${value%h} * 3600 ))" ;;
  esac

  case "$value" in
    ''|*[!0-9]*)
      echo "unsupported rollout timeout: $1" >&2
      return 2
      ;;
  esac

  echo "$value"
}

int_or_default() {
  value="$1"
  fallback="$2"
  if [ -z "$value" ]; then
    value="$fallback"
  fi
  case "$value" in
    *[!0-9]*)
      echo "$fallback"
      ;;
    *)
      echo "$value"
      ;;
  esac
}

deployment_snapshot() {
  snapshot_path='{.metadata.generation}{"|"}{.status.observedGeneration}{"|"}{.spec.replicas}{"|"}{.status.updatedReplicas}{"|"}{.status.readyReplicas}{"|"}{.status.availableReplicas}{"|"}{.status.unavailableReplicas}{"|"}{.status.replicas}'
  snapshot="$(
    run_kubectl --context "$context" -n "$namespace" get "deploy/$deployment" \
      -o "jsonpath=$snapshot_path" 2>/dev/null || true
  )"

  old_ifs="$IFS"
  IFS='|'
  read -r raw_generation raw_observed raw_desired raw_updated raw_ready raw_available raw_unavailable raw_total <<EOF || true
$snapshot
EOF
  IFS="$old_ifs"

  generation="$(int_or_default "${raw_generation:-}" 0)"
  observed="$(int_or_default "${raw_observed:-}" 0)"
  desired="$(int_or_default "${raw_desired:-}" 1)"
  updated="$(int_or_default "${raw_updated:-}" 0)"
  ready="$(int_or_default "${raw_ready:-}" 0)"
  available="$(int_or_default "${raw_available:-}" 0)"
  unavailable="$(int_or_default "${raw_unavailable:-}" 0)"
  total="$(int_or_default "${raw_total:-}" "$updated")"
}

wait_for_available_rollout() {
  timeout_seconds="$(duration_to_seconds "$timeout")" || return "$?"
  started_at="$(date +%s)"
  last_state=""

  while :; do
    deployment_snapshot

    state="desired=$desired updated=$updated ready=$ready available=$available unavailable=$unavailable observed=$observed generation=$generation"
    if [ "$state" != "$last_state" ]; then
      echo "availability gate for $namespace/$deployment: $state"
      last_state="$state"
    fi

    if [ "$observed" -ge "$generation" ] &&
       [ "$updated" -eq "$desired" ] &&
       [ "$ready" -ge "$desired" ] &&
       [ "$available" -ge "$desired" ] &&
       [ "$unavailable" -eq 0 ]; then
      old_replicas=0
      if [ "$total" -gt "$updated" ]; then
        old_replicas="$((total - updated))"
      fi

      if [ "$old_replicas" -gt 0 ]; then
        echo "deployment $namespace/$deployment is available; $old_replicas old replica(s) may still be terminating gracefully"
      else
        echo "deployment $namespace/$deployment is available"
      fi
      return 0
    fi

    now="$(date +%s)"
    if [ "$((now - started_at))" -ge "$timeout_seconds" ]; then
      echo "timed out waiting for deployment $namespace/$deployment availability gate: $state" >&2
      return 1
    fi

    sleep "$poll_interval"
  done
}

set +e
case "$wait_mode" in
  strict)
    run_kubectl --context "$context" -n "$namespace" rollout status "deploy/$deployment" --timeout="$timeout"
    ;;
  available)
    wait_for_available_rollout
    ;;
  *)
    echo "unsupported LOCAL_ROLLOUT_WAIT_MODE=$wait_mode (expected strict or available)" >&2
    exit 2
    ;;
esac
status="$?"
set -e

if [ "$status" -eq 0 ]; then
  exit 0
fi

echo
echo "rollout diagnostics for $namespace/$deployment"
echo "deployment:"
run_kubectl --context "$context" -n "$namespace" get "deploy/$deployment" -o wide || true
echo
echo "deployment conditions:"
run_kubectl --context "$context" -n "$namespace" describe "deploy/$deployment" || true

selector="$(
  # shellcheck disable=SC2016
  run_kubectl --context "$context" -n "$namespace" get "deploy/$deployment" \
    -o go-template='{{range $k,$v := .spec.selector.matchLabels}}{{printf "%s=%s," $k $v}}{{end}}' 2>/dev/null |
    sed 's/,$//'
)"

if [ -z "$selector" ]; then
  exit "$status"
fi

echo
echo "replicasets and pods for selector: $selector"
run_kubectl --context "$context" -n "$namespace" get rs,pods -l "$selector" -o wide || true

pods="$(
  run_kubectl --context "$context" -n "$namespace" get pods -l "$selector" \
    --sort-by=.metadata.creationTimestamp \
    -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null |
    tail -5
)"

for pod in $pods; do
  echo
  echo "pod diagnostics: $namespace/$pod"
  run_kubectl --context "$context" -n "$namespace" get "pod/$pod" -o wide || true
  run_kubectl --context "$context" -n "$namespace" describe "pod/$pod" | sed -n '/Events:/,$p' || true
  echo
  echo "current logs: $namespace/$pod"
  run_kubectl --context "$context" -n "$namespace" logs "pod/$pod" --all-containers=true --tail="$log_tail" || true
  echo
  echo "previous logs: $namespace/$pod"
  run_kubectl --context "$context" -n "$namespace" logs "pod/$pod" --all-containers=true --previous --tail="$log_tail" || true
done

exit "$status"
