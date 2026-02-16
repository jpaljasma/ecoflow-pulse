# Reference: Commands

## Go Commands

```bash
go test ./...
go run ./cmd/ecoflow-smoke
go run ./cmd/ecoflow-server
go run ./cmd/ecoflow-mqtt-sub
```

## Make Targets

```bash
make lint
make test
make bench
make build
make smoke
make mqtt
```

Notes:

- default `GOFLAGS` in `Makefile` include `-tags=moderncompress -mod=mod`,
- `make mqtt` exits cleanly on `q`/`Ctrl+C` and does not return non-zero on
  intentional stop.
