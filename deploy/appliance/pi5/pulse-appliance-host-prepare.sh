#!/usr/bin/env bash
set -euo pipefail

root="/"
dry_run=0
install_packages=1

usage() {
  cat <<'USAGE'
Usage: pulse-appliance-host-prepare.sh [--dry-run] [--root DIR] [--no-packages]

Applies Raspberry Pi 5 appliance host tuning files. Use --root for tests or
image preparation. Package installation and systemctl commands run only when
root is / and --no-packages is not set.
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run)
      dry_run=1
      ;;
    --root)
      root="${2:?--root requires a directory}"
      shift
      ;;
    --no-packages)
      install_packages=0
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

root="${root%/}"
[ -n "$root" ] || root="/"

target_path() {
  local path="${1#/}"
  if [ "$root" = "/" ]; then
    printf '/%s\n' "$path"
  else
    printf '%s/%s\n' "$root" "$path"
  fi
}

run() {
  if [ "$dry_run" -eq 1 ]; then
    printf 'DRY-RUN:'
    printf ' %q' "$@"
    printf '\n'
    return 0
  fi
  "$@"
}

write_file() {
  local path="$1"
  local mode="$2"
  local content="$3"
  local target
  target="$(target_path "$path")"
  if [ "$dry_run" -eq 1 ]; then
    printf 'DRY-RUN: write %s mode %s\n' "$target" "$mode"
    return 0
  fi
  install -d -m 0755 "$(dirname "$target")"
  printf '%s\n' "$content" >"$target"
  chmod "$mode" "$target"
}

merge_root_fstab_options() {
  local fstab
  fstab="$(target_path /etc/fstab)"
  if [ ! -f "$fstab" ]; then
    echo "missing fstab: $fstab" >&2
    return 1
  fi
  if [ "$dry_run" -eq 1 ]; then
    echo "DRY-RUN: merge noatime,errors=remount-ro into root fstab options at $fstab"
    return 0
  fi
  local tmp
  tmp="$(mktemp "${fstab}.XXXXXX")"
  awk '
    function hasopt(options, needle,   count, idx, parts) {
      count = split(options, parts, ",")
      for (idx = 1; idx <= count; idx++) {
        if (parts[idx] == needle) {
          return 1
        }
      }
      return 0
    }
    function withoutopt(options, needle,   count, idx, parts, out) {
      count = split(options, parts, ",")
      out = ""
      for (idx = 1; idx <= count; idx++) {
        if (parts[idx] == needle || parts[idx] == "") {
          continue
        }
        out = out == "" ? parts[idx] : out "," parts[idx]
      }
      return out
    }
    function addopt(options, needle) {
      if (hasopt(options, needle)) {
        return options
      }
      if (options == "" || options == "defaults") {
        return options "," needle
      }
      return options "," needle
    }
    /^[[:space:]]*#/ || NF < 4 {
      print
      next
    }
    $2 == "/" {
      $4 = withoutopt($4, "discard")
      $4 = addopt($4, "noatime")
      $4 = addopt($4, "errors=remount-ro")
      changed = 1
    }
    { print }
    END {
      if (changed != 1) {
        exit 3
      }
    }
  ' "$fstab" >"$tmp" || {
    rm -f "$tmp"
    echo "failed to update root fstab entry in $fstab" >&2
    return 1
  }
  mv "$tmp" "$fstab"
}

install_packages_if_enabled() {
  if [ "$root" != "/" ] || [ "$install_packages" -eq 0 ]; then
    return 0
  fi
  run apt update
  run apt full-upgrade -y
  run apt install -y ca-certificates curl gnupg jq unzip openssl \
    pciutils nvme-cli smartmontools bluez bluetooth rfkill \
    avahi-daemon chrony zram-tools
  run systemctl enable --now bluetooth avahi-daemon chrony fstrim.timer
  run rfkill unblock bluetooth
  if ! run systemctl disable --now dphys-swapfile; then
    echo "warning: dphys-swapfile was not disabled; service may be absent" >&2
  fi
}

install_packages_if_enabled

merge_root_fstab_options

write_file /etc/default/zramswap 0644 'ALGO=zstd
PERCENT=25
PRIORITY=100'

write_file /etc/sysctl.d/90-pulse-appliance.conf 0644 'vm.swappiness=10
vm.dirty_background_ratio=5
vm.dirty_ratio=10
fs.inotify.max_user_watches=524288
fs.inotify.max_user_instances=1024'

write_file /etc/systemd/journald.conf.d/90-pulse.conf 0644 '[Journal]
Storage=persistent
SystemMaxUse=512M
RuntimeMaxUse=128M
SystemKeepFree=2G
MaxRetentionSec=14day
Compress=yes'

write_file /etc/rancher/k3s/config.yaml 0644 'write-kubeconfig-mode: "0644"
disable:
  - traefik
  - servicelb
node-name: pulse-pi5
node-label:
  - pulse.appliance/local=true
kubelet-arg:
  - "system-reserved=cpu=500m,memory=1Gi,ephemeral-storage=12Gi"
  - "kube-reserved=cpu=300m,memory=512Mi,ephemeral-storage=8Gi"
  - "eviction-hard=memory.available<500Mi,nodefs.available<10%,imagefs.available<10%"
  - "eviction-minimum-reclaim=memory.available=256Mi,nodefs.available=2Gi,imagefs.available=2Gi"
  - "image-gc-high-threshold=70"
  - "image-gc-low-threshold=55"
  - "container-log-max-size=10Mi"
  - "container-log-max-files=3"'

echo "Pulse Pi 5 host preparation files applied."
