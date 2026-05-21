#!/usr/bin/env sh
set -eu

root_dir="$(cd "$(dirname "$0")/.." && pwd)"
script="$root_dir/scripts/local-dev-data-mode.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_mode() {
  expected="$1"
  shift
  actual="$("$@")"
  [ "$actual" = "$expected" ] || fail "expected $expected, got $actual"
}

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

cat >"$tmpdir/kubectl-public-cloud" <<'SH'
#!/usr/bin/env sh
case "$*" in
  *"deploy/pulse-platform-public-app"*) printf '%s' cloud ;;
  *) exit 1 ;;
esac
SH
chmod +x "$tmpdir/kubectl-public-cloud"

cat >"$tmpdir/kubectl-services-cloud" <<'SH'
#!/usr/bin/env sh
case "$*" in
  *"deploy/pulse-platform-public-app"*) printf '%s' local ;;
  *"configmap/pulse-services-runtime-env"*PROJECTION_KEY_PREFIX*) printf '%s' pulse:cloud-projection ;;
  *) exit 1 ;;
esac
SH
chmod +x "$tmpdir/kubectl-services-cloud"

assert_mode local-edge env DEV_DEPLOY_DATA_MODE=local-edge sh "$script"
assert_mode local-edge env DEV_DEPLOY_DATA_MODE=cloud sh "$script"
assert_mode local env DEV_DEPLOY_DATA_MODE=local sh "$script"
assert_mode local-edge env EXPO_PUBLIC_LOCAL_DATA_PLANE=cloud sh "$script"
assert_mode local-edge env KUBECTL="$tmpdir/kubectl-public-cloud" DEV_DEPLOY_DATA_MODE=auto sh "$script"
assert_mode local-edge env KUBECTL="$tmpdir/kubectl-services-cloud" DEV_DEPLOY_DATA_MODE=auto sh "$script"
assert_mode local env KUBECTL=/bin/false DEV_DEPLOY_DATA_MODE=auto sh "$script"
