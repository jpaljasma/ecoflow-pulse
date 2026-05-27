# How To Discover EcoFlow BLE Devices

Use `cmd/ecoflow-ble-discover` to scan nearby Bluetooth Low Energy
advertisements and identify EcoFlow-looking devices before building a local BLE
telemetry collector.

By default, the command scans for five seconds, shows a numbered list of
candidate devices, lets you select one device, and then probes its BLE GATT
services/characteristics. It prints redacted identifiers by default. The probe
discovers capabilities and safe initial metrics such as RSSI, advertised service
count, manufacturer data block count, and standard GATT Battery Service or
Device Information values when the device exposes them. It does not yet decode
EcoFlow proprietary telemetry frames.

## Run Discovery And Probe

```bash
make ecoflow-ble-discover
```

Example output:

```text
summary seen=1 ecoflow=1 elapsed=5s
discovered devices:
1) address=A1B2...4455 name="EF-PR...0000" rssi=-54 model="EcoFlow DELTA 3 1000 Air" prefix=PR1W packets=v3 services=1 manufacturer=1
select device [1-1, empty to skip]:
probing address=A1B2...4455 name="EF-PR...0000" model="EcoFlow DELTA 3 1000 Air"
capabilities services=3 characteristics=8 mtus=185
service uuid=0000180a-0000-1000-8000-00805f9b34fb characteristics=3
metric rssi_dbm=-54 unit="dBm" source=advertisement
metric advertised_services=1 source=advertisement
metric manufacturer_data_blocks=1 source=advertisement
metric prefix=PR1W source=advertisement
```

Press Enter at the selection prompt to skip the probe. Use `-select=1` to select
the first listed device without an interactive prompt; the selector also accepts
`first`, a raw BLE address, a local name, or an unambiguous model prefix.

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
followed by a `"type":"summary"` object. JSON mode does not prompt
interactively; add `-select=1` or another selector to emit a `"type":"probe"`
object after the summary.

## Useful Flags

- `-duration=5s`: scan duration. Use `0` to scan until interrupted.
- `-redact=true`: redact BLE addresses and EcoFlow local names.
- `-all=false`: include only likely EcoFlow devices.
- `-min-rssi=-100`: filter weak advertisements. Use `0` to disable filtering.
- `-format=text`: output `text` or `json`.
- `-name-prefix=EF-`: extra local-name prefix treated as an EcoFlow candidate.
- `-scan-only=false`: print advertisements without prompting or connecting.
- `-select=`: select by menu number, `first`, BLE address, local name, or
  unambiguous prefix without prompting.
- `-probe-timeout=10s`: maximum time to spend probing the selected device.

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
