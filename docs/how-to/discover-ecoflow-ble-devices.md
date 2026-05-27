# How To Discover EcoFlow BLE Devices

Use `cmd/ecoflow-ble-discover` to scan nearby Bluetooth Low Energy
advertisements and identify EcoFlow-looking devices before building a local BLE
telemetry collector.

The command does not connect to the device and does not read telemetry. It only
listens for advertisements, infers known EcoFlow model prefixes when possible,
and prints redacted identifiers by default.

## Run A Redacted Scan

```bash
make ecoflow-ble-discover ECOFLOW_BLE_DISCOVER_ARGS='-duration=20s'
```

Example output:

```text
device address=A1B2...4455 name="EF-PR...0498" rssi=-54 model="EcoFlow DELTA 3 1000 Air (10ms UPS)" serial_prefix=PR12 packets=v3
summary seen=1 ecoflow=1 elapsed=20s
```

## Reveal Raw Local Identifiers

Use this only for local troubleshooting. BLE addresses, local names, and serial
fragments should not be pasted into PRs, issues, or logs that leave your
machine.

```bash
make ecoflow-ble-discover ECOFLOW_BLE_DISCOVER_ARGS='-duration=20s -redact=false'
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
followed by a `"type":"summary"` object.

## Useful Flags

- `-duration=15s`: scan duration. Use `0` to scan until interrupted.
- `-redact=true`: redact BLE addresses and EcoFlow local names.
- `-all=false`: include only likely EcoFlow devices.
- `-min-rssi=-100`: filter weak advertisements. Use `0` to disable filtering.
- `-format=text`: output `text` or `json`.
- `-name-prefix=EF-`: extra local-name prefix treated as an EcoFlow candidate.

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
- EcoFlow devices generally allow only one BLE connection at a time. This
  discovery command only scans advertisements, but later telemetry collectors
  that connect to the device can conflict with the EcoFlow mobile app.
