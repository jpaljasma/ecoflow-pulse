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
kubectl_bin="${KUBECTL:-kubectl}"
log_tail="${ROLLOUT_DEBUG_LOG_TAIL:-160}"

set +e
"$kubectl_bin" --context "$context" -n "$namespace" rollout status "deploy/$deployment" --timeout="$timeout"
status="$?"
set -e

if [ "$status" -eq 0 ]; then
  exit 0
fi

echo
echo "rollout diagnostics for $namespace/$deployment"
echo "deployment:"
"$kubectl_bin" --context "$context" -n "$namespace" get "deploy/$deployment" -o wide || true
echo
echo "deployment conditions:"
"$kubectl_bin" --context "$context" -n "$namespace" describe "deploy/$deployment" || true

selector="$(
  # shellcheck disable=SC2016
  "$kubectl_bin" --context "$context" -n "$namespace" get "deploy/$deployment" \
    -o go-template='{{range $k,$v := .spec.selector.matchLabels}}{{printf "%s=%s," $k $v}}{{end}}' 2>/dev/null |
    sed 's/,$//'
)"

if [ -z "$selector" ]; then
  exit "$status"
fi

echo
echo "replicasets and pods for selector: $selector"
"$kubectl_bin" --context "$context" -n "$namespace" get rs,pods -l "$selector" -o wide || true

pods="$(
  "$kubectl_bin" --context "$context" -n "$namespace" get pods -l "$selector" \
    --sort-by=.metadata.creationTimestamp \
    -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null |
    tail -5
)"

for pod in $pods; do
  echo
  echo "pod diagnostics: $namespace/$pod"
  "$kubectl_bin" --context "$context" -n "$namespace" get "pod/$pod" -o wide || true
  "$kubectl_bin" --context "$context" -n "$namespace" describe "pod/$pod" | sed -n '/Events:/,$p' || true
  echo
  echo "current logs: $namespace/$pod"
  "$kubectl_bin" --context "$context" -n "$namespace" logs "pod/$pod" --all-containers=true --tail="$log_tail" || true
  echo
  echo "previous logs: $namespace/$pod"
  "$kubectl_bin" --context "$context" -n "$namespace" logs "pod/$pod" --all-containers=true --previous --tail="$log_tail" || true
done

exit "$status"
