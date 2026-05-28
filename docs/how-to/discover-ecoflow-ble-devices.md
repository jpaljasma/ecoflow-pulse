# How To Discover EcoFlow BLE Devices

Use `cmd/ecoflow-ble-discover` to scan nearby Bluetooth Low Energy
advertisements and identify EcoFlow-looking devices before building a local BLE
telemetry collector.

By default, the command scans for five seconds, shows a numbered list of
candidate devices, automatically probes supported DELTA 3 and RIVER 3-family
devices in parallel, and prints a compact ASCII refresh table for each device
as new metrics arrive. It writes the detailed raw probe event stream to
`.tmp/ecoflow-ble-discover-raw.jsonl` and overwrites that file on every run.
Auto-probe mode runs until you press `Ctrl-C`; on shutdown it cancels the probe
contexts so notification subscriptions are disabled and BLE connections are
closed by the normal probe cleanup path.
The probe discovers capabilities and safe initial metrics such as RSSI,
advertised service count, manufacturer data block count, and standard GATT
Battery Service or Device Information values when the device exposes them. It
briefly subscribes to EcoFlow-looking notification characteristics and tries to
classify EcoFlow BLE frames. Shared v3 display-property uploads are decoded into
initial power metrics such as load, AC input, AC output, PV input, USB output,
battery power, DC charge type, and battery state of charge when the device emits
plain or wrapped plaintext packets.

## Run Discovery And Probe

```bash
make ecoflow-ble-discover
```

Example output:

```text
summary seen=2 ecoflow=2 elapsed=5s
discovered devices:
1) address=A1B2...4455 name="EF-PR...0000" rssi=-54 model="EcoFlow DELTA 3 1000 Air" prefix=PR1W packets=v3 services=1 manufacturer=1
2) address=B2C3...5566 name="EF-R3...0000" rssi=-60 model="EcoFlow RIVER 3 Plus (270Wh)" prefix=R3PG packets=v3 services=1 manufacturer=1
auto probing supported devices=2
raw_output path=.tmp/ecoflow-ble-discover-raw.jsonl
probing address=A1B2...4455 name="EF-PR...0000" model="EcoFlow DELTA 3 1000 Air"
probing address=B2C3...5566 name="EF-R3...0000" model="EcoFlow RIVER 3 Plus (270Wh)"
+---------------------+-------------------------------+-------------------------------+
| Metric              | EF-PR...0000                  | EF-R3...0000                  |
+---------------------+-------------------------------+-------------------------------+
| Device              | EF-PR...0000                  | EF-R3...0000                  |
| Model               | EcoFlow DELTA 3 1000 Air      | EcoFlow RIVER 3 Plus (270Wh)  |
| Address             | A1B2...4455                   | B2C3...5566                   |
| Update              | 2                             | 2                             |
| Packets             | 1                             | 1                             |
| Current load        | 80 W                          | 32 W                          |
| Solar in            | 55 W                          | 0 W                           |
| Battery charge      | 76%                           | 91%                           |
| ETA                 | discharge: 4h 12m             | discharge: 8h 30m             |
| Services            | 2                             | 1                             |
| Characteristics     | 4                             | 2                             |
| MTUs                | 497                           | 497                           |
| RSSI                | -54 dBm                       | -60 dBm                       |
| Auth                | ok                            | ok                            |
| Total input         | 145.5 W                       | 0 W                           |
| AC input            | 123 W                         | 0 W                           |
| AC output           | 80 W                          | 32 W                          |
| Battery power       | 25 W                          | 32 W                          |
| DC charging         | solar                         | off                           |
| AC charger          | true                          | false                         |
| AC output enabled   | true                          | true                          |
+---------------------+-------------------------------+-------------------------------+
```

Use `-select=1` to probe one listed device with the verbose legacy text output;
the selector also accepts `first`, a raw BLE address, a local name, or an
unambiguous model prefix. Use `-select=auto`, `-select=supported`, or
`-select=all` to explicitly request the default parallel supported-device
probe.

For compact refresh tables, each supported device gets a fixed-width column.
`Update` is the number of metric refreshes observed for that device and
`Packets` is the count of decoded EcoFlow packet summaries seen by the summary
stream. The headline rows show current load, solar input, battery charge, and
ETA as soon as those metrics are available; missing values stay as `-` so the
table shape remains stable while data arrives. In the raw JSONL file, metric
`source` is the transport where the value arrived and `decoder` is the parser
that interpreted the packet.

If the raw file only shows `frame=ecoflow_enc_packet` with
`encrypted_or_unknown_payload`, the local BLE path is visible but telemetry is
behind EcoFlow BLE session authentication. The raw JSONL file is local
reverse-engineering output and should not be pasted into issues, PRs, or shared
logs.

## Run An Active BLE Probe

Some EcoFlow devices do not emit unsolicited BLE notifications after a passive
subscribe. For those devices, use an active probe to write a small protocol
request after notifications are enabled, then listen for the response:

```bash
make ecoflow-ble-discover ECOFLOW_BLE_DISCOVER_ARGS='-duration=5s -select 1 -probe-timeout=20s -listen-duration=8s -active-probe=auto -ble-transport=both'
```

The default auto-probe writes transmitted and received raw frame hex plus
decoded inner payload hex to `-raw-output` where the frame wrapper is
understood. With an explicit single-device `-select`, add `-raw-notifications`
only for local protocol debugging when you also want that hex in stdout. Treat
raw output as local-only because decoded buffers may reveal device-linked
identifiers.

The default supported-device auto-probe uses `-active-probe=auto` internally
unless you explicitly pass `-active-probe=none`. It also listens until
interruption unless you pass an explicit `-listen-duration`.

For encrypted type-7 devices, `-active-probe=auto` starts with the EcoFlow ECDH
public-key exchange. When the device answers, the probe derives the initial
shared AES key in memory, sends the follow-up `session_key_info_request`,
derives the per-session key from the returned seed, and sends encrypted
`auth_status_request`. It prints a `session_key_info_decrypted` line if it can
decrypt the returned key info payload and `type7_packet_decrypted` lines for
later encrypted packets. It does not print derived AES keys.

If an authentication response reports anything other than `auth_result=ok`, the
probe treats that as a failed BLE authentication attempt, prints the latest
probe output, cancels any parallel device probes, and exits with an error.

To continue from auth status into full BLE authentication, provide your EcoFlow
user ID with `-auth-user-id` or `ECOFLOW_BLE_USER_ID`. The command uses the
device serial from EcoFlow manufacturer data when present; override it with
`-auth-device-serial` or `ECOFLOW_BLE_DEVICE_SERIAL` only for devices whose
advertisement does not include a usable serial. The auth payload is redacted
from decoded output even when `-raw-notifications` is enabled.

## Run A Scan Without Connecting

```bash
make ecoflow-ble-discover ECOFLOW_BLE_DISCOVER_ARGS='-scan-only -duration=20s'
```

Scan-only mode prints matching advertisements as they arrive and does not open a
BLE connection.

## Reveal Raw Local Identifiers

Use this only for local troubleshooting. BLE addresses, local names, and serial
fragments should not be pasted into PRs, issues, or logs that leave your
machine.

```bash
make ecoflow-ble-discover ECOFLOW_BLE_DISCOVER_ARGS='-redact=false'
```

## Include Nearby Non-EcoFlow Devices

By default, the scanner only prints likely EcoFlow advertisements. To inspect
everything the adapter sees:

```bash
make ecoflow-ble-discover ECOFLOW_BLE_DISCOVER_ARGS='-duration=10s -all'
```

## Emit JSON Lines

```bash
make ecoflow-ble-discover ECOFLOW_BLE_DISCOVER_ARGS='-duration=20s -format=json'
```

Each discovered device is emitted as a JSON object with `"type":"device"`,
followed by a `"type":"summary"` object. JSON mode does not prompt or auto-probe
interactively; add `-select=1` or another single-device selector to emit a
`"type":"probe"` object after the summary.

## Useful Flags

- `-duration=5s`: scan duration. Use `0` to scan until interrupted.
- `-redact=true`: redact BLE addresses and EcoFlow local names.
- `-all=false`: include only likely EcoFlow devices.
- `-min-rssi=-100`: filter weak advertisements. Use `0` to disable filtering.
- `-format=text`: output `text` or `json`.
- `-name-prefix=EF-`: extra local-name prefix treated as an EcoFlow candidate.
- `-scan-only=false`: print advertisements without prompting or connecting.
- `-select=`: default empty value auto-probes supported DELTA 3 and RIVER
  3-family devices in parallel. Select by menu number, `first`, BLE address,
  local name, or unambiguous prefix to force one verbose probe.
- `-probe-timeout=10s`: maximum time to spend probing an explicit selected
  device. Auto-probe mode runs until interrupted by default; use a positive
  explicit value to bound it.
- `-listen-duration=5s`: how long to listen for EcoFlow notification frames
  after service discovery for explicit selected probes. Auto-probe mode listens
  until interrupted by default; pass an explicit value to bound it, or `0` to
  disable notification listening.
- `-active-probe=none`: optional active probe to send after notifications are
  enabled. Use `auto`, `ecdh`, or `auth-status` while reverse engineering.
- `-ble-transport=auto`: active-probe transport. Use `rfcomm`, `alt`, or `both`
  to force a specific EcoFlow write/notify pair.
- `-auth-user-id=`: EcoFlow user ID for BLE authentication. Defaults to
  `ECOFLOW_BLE_USER_ID`.
- `-auth-device-serial=`: device serial override for BLE authentication.
  Defaults to manufacturer advertisement serial, then
  `ECOFLOW_BLE_DEVICE_SERIAL`.
- `-raw-notifications=false`: include raw and decoded probe buffer hex for local
  protocol debugging. Leave disabled for shareable output.
- `-raw-output=.tmp/ecoflow-ble-discover-raw.jsonl`: JSONL file for detailed
  raw probe events in auto-probe mode. It is overwritten on every run; use an
  empty value to disable.

## Bluetooth Notes

- On macOS, prefer the `make ecoflow-ble-discover` target. It embeds
  `cmd/ecoflow-ble-discover/Info.plist` into the binary so CoreBluetooth has
  the required `NSBluetoothAlwaysUsageDescription` usage string and then
  ad-hoc signs the binary. Direct `go run ./cmd/ecoflow-ble-discover` can be
  killed by macOS before Go handles the error when the launcher lacks Bluetooth
  usage metadata.
- On macOS, the first successful run may prompt for Bluetooth permission.
- If macOS still aborts because the parent app is treated as the responsible
  process, run the built `bin/ecoflow-ble-discover` binary from a normal
  terminal session that can be granted Bluetooth permission in System Settings.
- On Linux, the scanner uses the host BlueZ stack through the Go Bluetooth
  adapter library. The user may need permissions for BLE scanning.
- EcoFlow devices generally allow only one BLE connection at a time. The default
  probe connects to the selected device and can conflict with the EcoFlow mobile
  app while it is running. Use `-scan-only` when you only need advertisements.
