//go:build integration

package pipelineintegration

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/archiveworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/integrationtest"
	"github.com/jpaljasma/ecoflow-pulse/internal/projectionworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/rollupworker"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	valkey "github.com/valkey-io/valkey-go"
)

func TestTelemetryPipelineEndToEndIntegration(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PIPELINE_INTEGRATION")) != "1" {
		t.Skip("set PIPELINE_INTEGRATION=1 to run telemetry pipeline integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	stack, err := integrationtest.StartStack(ctx, integrationtest.DefaultStackOptions())
	if err != nil {
		t.Fatalf("start integration stack: %v", err)
	}
	t.Cleanup(func() {
		_ = stack.Terminate(context.Background())
	})

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	subjectCfg := telemetrybus.SubjectConfig{Prefix: "pulse", ShardCount: 8}

	natsConn, err := telemetrybus.DialNATS(log, telemetrybus.DefaultNATSConnConfig([]string{stack.NATSURL}))
	if err != nil {
		t.Fatalf("dial nats: %v", err)
	}
	t.Cleanup(func() { natsConn.Close() })

	streamCfg := telemetrybus.DefaultJetStreamIngestBootstrapConfig()
	streamCfg.Replicas = 1
	if err := telemetrybus.EnsureJetStreamIngestStream(ctx, natsConn, subjectCfg, streamCfg); err != nil {
		t.Fatalf("ensure ingest stream: %v", err)
	}

	valkeyClient, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:  []string{stack.ValkeyAddress},
		DisableCache: true,
	})
	if err != nil {
		t.Fatalf("create valkey client: %v", err)
	}
	t.Cleanup(valkeyClient.Close)

	snapshotStore, err := projectionworker.NewValkeySnapshotStore(
		valkeyClient,
		projectionworker.DefaultValkeySnapshotStoreConfig(),
	)
	if err != nil {
		t.Fatalf("create snapshot store: %v", err)
	}

	rollupStore, err := rollupworker.NewPostgresStore(stack.PostgresDSN)
	if err != nil {
		t.Fatalf("create rollup store: %v", err)
	}
	t.Cleanup(func() {
		closeErr := rollupStore.Close()
		if closeErr != nil {
			t.Errorf("close rollup store: %v", closeErr)
		}
	})

	manifestStore, err := archiveworker.NewPostgresManifestStore(stack.PostgresDSN)
	if err != nil {
		t.Fatalf("create archive manifest store: %v", err)
	}
	t.Cleanup(func() {
		closeErr := manifestStore.Close()
		if closeErr != nil {
			t.Errorf("close archive manifest store: %v", closeErr)
		}
	})

	objectCfg := archiveworker.DefaultMinIOObjectStoreConfig()
	objectCfg.Endpoint = stack.MinIOEndpoint
	objectCfg.AccessKeyID = stack.MinIOUser
	objectCfg.SecretAccessKey = stack.MinIOPass
	objectCfg.Region = stack.MinIORegion
	objectStore, err := archiveworker.NewMinIOObjectStore(objectCfg)
	if err != nil {
		t.Fatalf("create archive object store: %v", err)
	}

	projectionCfg := projectionworker.DefaultConfig()
	projectionCfg.SubjectConfig = subjectCfg
	projectionCfg.ProcessTimeout = 1500 * time.Millisecond

	rollupCfg := rollupworker.DefaultConfig()
	rollupCfg.SubjectConfig = subjectCfg
	rollupCfg.ProcessTimeout = 1500 * time.Millisecond

	archiveCfg := archiveworker.DefaultConfig()
	archiveCfg.SubjectConfig = subjectCfg
	archiveCfg.ProcessTimeout = 1500 * time.Millisecond
	archiveCfg.FlushTimeout = 3 * time.Second
	archiveCfg.FlushInterval = 300 * time.Millisecond
	archiveCfg.MaxRecordsPerPart = 1
	archiveCfg.MaxBytesPerPart = 1024
	archiveCfg.ObjectBucket = "pulse-telemetry-raw"

	projection, err := projectionworker.New(log, natsConn, snapshotStore, projectionCfg)
	if err != nil {
		t.Fatalf("create projection worker: %v", err)
	}
	rollup, err := rollupworker.New(log, natsConn, rollupStore, rollupCfg)
	if err != nil {
		t.Fatalf("create rollup worker: %v", err)
	}
	archive, err := archiveworker.New(log, natsConn, objectStore, archiveCfg, archiveworker.WithManifestStore(manifestStore))
	if err != nil {
		t.Fatalf("create archive worker: %v", err)
	}

	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()
	errCh := make(chan error, 3)
	runWorker := func(name string, run func(context.Context) error) {
		go func() {
			if runErr := run(workerCtx); runErr != nil && !errors.Is(runErr, context.Canceled) {
				errCh <- fmt.Errorf("%s worker: %w", name, runErr)
			}
		}()
	}
	runWorker("projection", projection.Run)
	runWorker("rollup", rollup.Run)
	runWorker("archive", archive.Run)

	publisher, err := telemetrybus.NewNATSEnvelopePublisherWithOptions(
		natsConn,
		subjectCfg,
		telemetrybus.NATSEnvelopePublisherOptions{
			UseJetStream:               true,
			PublishTimeout:             2 * time.Second,
			PublishMaxRetries:          3,
			PublishRetryInitialBackoff: 50 * time.Millisecond,
		},
	)
	if err != nil {
		t.Fatalf("create publisher: %v", err)
	}
	t.Cleanup(func() {
		closeErr := publisher.Close()
		if closeErr != nil {
			t.Errorf("close publisher: %v", closeErr)
		}
	})

	deviceID := uuid.NewString()
	providerSN := "PIPELINEINT0001"
	now := time.Now().UTC().Truncate(time.Second)
	for i := range 2 {
		payload := map[string]any{
			"params": map[string]any{
				"wattsInSum":     240 + i*5,
				"wattsOutSum":    120 + i*2,
				"pv1ChargeWatts": 80 + i*3,
				"f32ShowSoc":     55.5 + float64(i),
				"temp":           21.5 + float64(i),
			},
			"pvW": 80 + i*3,
		}
		encodedPayload, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			t.Fatalf("marshal payload: %v", marshalErr)
		}
		ts := now.Add(time.Duration(i) * time.Second).UnixMilli()
		envelope := &envelopev1.TelemetryEnvelope{
			EnvelopeId:         uuid.NewString(),
			EnvelopeVersion:    1,
			DeviceId:           deviceID,
			EcoflowSn:          providerSN,
			Shard:              telemetrybus.ShardForDevice(deviceID, subjectCfg.ShardCount),
			ShardCount:         subjectCfg.ShardCount,
			MessageId:          fmt.Sprintf("msg-%d", i+1),
			DeviceTimeUnixMs:   ts,
			ObservedTimeUnixMs: ts,
			IngestedTimeUnixMs: ts + 5,
			SourceKind:         envelopev1.SourceKind_SOURCE_KIND_MQTT_STATUS,
			Source:             "mqtt",
			TypeCode:           "telemetry",
			PayloadType:        "ecoflow.mqtt.raw",
			PayloadVersion:     1,
			PayloadEncoding:    envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
			Payload:            encodedPayload,
			Labels: map[string]string{
				"provider": "ecoflow",
			},
		}
		if err := telemetrybus.PublishEnvelope(ctx, publisher, envelope); err != nil {
			t.Fatalf("publish envelope %d: %v", i+1, err)
		}
	}

	assertEventually(t, 35*time.Second, 300*time.Millisecond, func() (bool, error) {
		select {
		case runErr := <-errCh:
			return false, runErr
		default:
		}
		snap, getErr := snapshotStore.GetSnapshot(ctx, deviceID, providerSN)
		if getErr != nil {
			return false, getErr
		}
		if snap == nil || len(snap.Metrics) == 0 {
			return false, nil
		}
		return snap.Metrics["params.wattsInSum"] > 0, nil
	}, "projection snapshot")

	assertEventually(t, 35*time.Second, 300*time.Millisecond, func() (bool, error) {
		select {
		case runErr := <-errCh:
			return false, runErr
		default:
		}
		count, queryErr := countRows(ctx, stack.PostgresDSN,
			`SELECT COUNT(*) FROM telemetry_rollup_minute WHERE provider='ecoflow' AND provider_device_id=$1 AND device_id=$2`,
			strings.ToUpper(providerSN), deviceID,
		)
		if queryErr != nil {
			return false, queryErr
		}
		return count > 0, nil
	}, "minute rollup row")

	assertEventually(t, 35*time.Second, 300*time.Millisecond, func() (bool, error) {
		select {
		case runErr := <-errCh:
			return false, runErr
		default:
		}
		count, queryErr := countRows(ctx, stack.PostgresDSN,
			`SELECT COUNT(*) FROM archive_object_manifest WHERE provider='ecoflow' AND $1 = ANY(provider_device_ids)`,
			strings.ToUpper(providerSN),
		)
		if queryErr != nil {
			return false, queryErr
		}
		return count > 0, nil
	}, "archive manifest row")

	assertEventually(t, 35*time.Second, 300*time.Millisecond, func() (bool, error) {
		select {
		case runErr := <-errCh:
			return false, runErr
		default:
		}
		exists, listErr := objectExists(ctx, stack, "pulse-telemetry-raw", "raw/")
		if listErr != nil {
			return false, listErr
		}
		return exists, nil
	}, "archive object")
}

func countRows(ctx context.Context, dsn, query string, args ...any) (int, error) {
	db, err := sql.Open("pgx", strings.TrimSpace(dsn))
	if err != nil {
		return 0, fmt.Errorf("open postgres: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()
	var count int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func objectExists(ctx context.Context, stack *integrationtest.Stack, bucket, prefix string) (bool, error) {
	client, err := minio.New(strings.TrimSpace(stack.MinIOEndpoint), &minio.Options{
		Creds:  credentials.NewStaticV4(stack.MinIOUser, stack.MinIOPass, ""),
		Secure: false,
		Region: stack.MinIORegion,
	})
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for object := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    strings.TrimSpace(prefix),
		Recursive: true,
	}) {
		if object.Err != nil {
			return false, object.Err
		}
		if strings.TrimSpace(object.Key) != "" {
			return true, nil
		}
	}
	return false, nil
}

func assertEventually(
	t *testing.T,
	timeout time.Duration,
	interval time.Duration,
	check func() (bool, error),
	label string,
) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		ok, err := check()
		if err != nil {
			lastErr = err
		}
		if ok {
			return
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				t.Fatalf("%s not satisfied before timeout: %v", label, lastErr)
			}
			t.Fatalf("%s not satisfied before timeout", label)
		}
		time.Sleep(interval)
	}
}
