# syntax=docker/dockerfile:1.7

FROM golang:1.26 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,id=ecoflow-pulse-go-mod-1.26,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,id=ecoflow-pulse-go-build-1.26,target=/root/.cache/go-build,sharing=locked \
    go mod download

COPY . .

RUN --mount=type=cache,id=ecoflow-pulse-go-mod-1.26,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,id=ecoflow-pulse-go-build-1.26,target=/root/.cache/go-build,sharing=locked \
    set -euo pipefail; \
    export CGO_ENABLED=0 GOOS=linux GOARCH=amd64; \
    for cmd in \
        ecoflow-ingest-worker \
        ecoflow-inference-worker \
        ecoflow-rollup-worker \
        ecoflow-projection-worker \
        ecoflow-archive-worker \
        ecoflow-gap-detector \
        ecoflow-gap-repair-worker \
        ecoflow-grpc-api; do \
        go build -trimpath -tags=moderncompress -o "/out/${cmd}" "./cmd/${cmd}"; \
    done

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/ /app/

USER nonroot:nonroot
