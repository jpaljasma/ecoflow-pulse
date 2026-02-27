FROM golang:1.26 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -tags=moderncompress -o /out/ecoflow-ingest-worker ./cmd/ecoflow-ingest-worker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -tags=moderncompress -o /out/ecoflow-rollup-worker ./cmd/ecoflow-rollup-worker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -tags=moderncompress -o /out/ecoflow-projection-worker ./cmd/ecoflow-projection-worker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -tags=moderncompress -o /out/ecoflow-archive-worker ./cmd/ecoflow-archive-worker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -tags=moderncompress -o /out/ecoflow-gap-detector ./cmd/ecoflow-gap-detector
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -tags=moderncompress -o /out/ecoflow-gap-repair-worker ./cmd/ecoflow-gap-repair-worker

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/ /app/

USER nonroot:nonroot
