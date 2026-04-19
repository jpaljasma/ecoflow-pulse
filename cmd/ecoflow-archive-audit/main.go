package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/archiveaudit"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

func main() {
	var (
		from               string
		to                 string
		bucket             string
		prefix             string
		maxObjects         int
		objectProvider     string
		objectEndpoint     string
		objectAccessKey    string
		objectSecretKey    string
		objectRegion       string
		objectSecure       bool
		objectGCSProjectID string
		manifestDSN        string
	)

	flag.StringVar(&from, "from", "", "inclusive UTC timestamp, RFC3339")
	flag.StringVar(&to, "to", "", "exclusive UTC timestamp, RFC3339")
	flag.StringVar(&bucket, "archive-bucket", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_BUCKET", "pulse-telemetry-raw"), "archive bucket")
	flag.StringVar(&prefix, "archive-prefix", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_PREFIX", "raw"), "archive prefix")
	flag.IntVar(&maxObjects, "max-objects", 0, "maximum objects to inspect (0 = all)")
	objectDefaults := replaycli.DefaultObjectReaderConfig()
	flag.StringVar(&objectProvider, "object-provider", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_PROVIDER", string(objectDefaults.Provider)), "object store provider: minio|gcs")
	flag.StringVar(&objectEndpoint, "object-endpoint", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ENDPOINT", objectDefaults.Endpoint), "object store endpoint")
	flag.StringVar(&objectAccessKey, "object-access-key", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_ACCESS_KEY", objectDefaults.AccessKeyID), "object store access key")
	flag.StringVar(&objectSecretKey, "object-secret-key", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_SECRET_KEY", objectDefaults.SecretAccessKey), "object store secret key")
	flag.StringVar(&objectRegion, "object-region", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_REGION", objectDefaults.Region), "object store region")
	flag.BoolVar(&objectSecure, "object-secure", runtimecfg.Bool("ARCHIVE_OBJECT_SECURE", objectDefaults.Secure), "object store tls")
	flag.StringVar(&objectGCSProjectID, "object-gcs-project-id", runtimecfg.EnvOrDefault("ARCHIVE_OBJECT_GCS_PROJECT_ID", objectDefaults.GCSProjectID), "optional GCS project id for logging or bucket auto-create")
	flag.StringVar(&manifestDSN, "manifest-dsn", strings.TrimSpace(runtimecfg.EnvOrDefault("CONTROL_PLANE_DB_DSN", "")), "Postgres DSN for archive manifest index")
	flag.Parse()

	logCfg := pulselog.DefaultServiceConfig("archive-audit")
	logCfg.Level = pulselog.ParseLevel(os.Getenv("LOG_LEVEL"), slog.LevelInfo)
	log, asyncLogHandler, err := pulselog.BuildServiceLogger(logCfg)
	if err != nil {
		_, _ = os.Stderr.WriteString("init logger failed: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer func() {
		if asyncLogHandler != nil {
			asyncLogHandler.Close()
		}
	}()

	fromTime, err := parseRequiredRFC3339("from", from)
	if err != nil {
		log.Error("invalid from", slog.String("error", err.Error()))
		os.Exit(1)
	}
	toTime, err := parseRequiredRFC3339("to", to)
	if err != nil {
		log.Error("invalid to", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if !toTime.After(fromTime) {
		log.Error("invalid window", slog.String("error", "to must be after from"))
		os.Exit(1)
	}
	if strings.TrimSpace(manifestDSN) == "" {
		log.Error("manifest dsn is required")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	manifestStore, err := replaycli.NewPostgresManifestStore(manifestDSN)
	if err != nil {
		log.Error("init manifest store failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() { _ = manifestStore.Close() }()

	manifestObjects, err := manifestStore.ListByFleetRange(ctx, replaycli.FleetQuery{
		FromUnixMS:         fromTime.UnixMilli(),
		ToUnixMS:           toTime.UnixMilli(),
		MaxObjectsReturned: maxObjects,
	})
	if err != nil {
		log.Error("list manifest objects failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	directObjects, err := replaycli.ListObjectsDirect(
		ctx,
		replaycli.ObjectReaderConfig{
			Provider:        replaycli.ObjectProvider(objectProvider),
			Endpoint:        objectEndpoint,
			AccessKeyID:     objectAccessKey,
			SecretAccessKey: objectSecretKey,
			Region:          objectRegion,
			Secure:          objectSecure,
			GCSProjectID:    objectGCSProjectID,
		},
		bucket,
		prefix,
		fromTime,
		toTime,
		maxObjects,
	)
	if err != nil {
		log.Error("list direct archive objects failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	report := archiveaudit.Compare(manifestObjects, directObjects)
	log.Info("archive audit summary",
		slog.String("from", fromTime.Format(time.RFC3339)),
		slog.String("to", toTime.Format(time.RFC3339)),
		slog.Int("manifest_objects", report.ManifestObjects),
		slog.Int("direct_objects", report.DirectObjects),
		slog.Int("missing_in_archive", report.MissingInArchiveCount),
		slog.Int("missing_in_manifest", report.MissingInManifestCount),
	)

	if report.MissingInArchiveCount > 0 {
		for _, key := range report.MissingInArchiveKeys {
			log.Error("archive manifest references missing object", slog.String("object", key))
		}
	}
	if report.MissingInManifestCount > 0 {
		for _, key := range report.MissingInManifestKeys {
			log.Error("archive object missing manifest row", slog.String("object", key))
		}
	}
	if !report.Healthy() {
		os.Exit(1)
	}
}

func parseRequiredRFC3339(name, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be RFC3339: %w", name, err)
	}
	return parsed.UTC(), nil
}
