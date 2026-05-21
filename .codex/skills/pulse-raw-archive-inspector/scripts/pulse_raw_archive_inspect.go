package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"google.golang.org/protobuf/proto"
)

const (
	defaultNamespace     = "pulse-services"
	defaultRuntimeSecret = "pulse-services-runtime-secret"
	defaultRuntimeConfig = "pulse-services-runtime-env"
	defaultCloudProject  = "ecoflow-pulse-dev-260221-01"
)

type config struct {
	provider        string
	family          string
	search          string
	payloadType     string
	hours           float64
	from            string
	to              string
	dbDSN           string
	objectProvider  string
	objectEndpoint  string
	objectAccessKey string
	objectSecretKey string
	objectRegion    string
	objectSecure    string
	gcsProjectID    string
	preset          string
	kubeContext     string
	kubeNamespace   string
	runtimeSecret   string
	runtimeConfig   string
	loadRuntime     bool
	rewriteDocker   bool
	maxObjects      int
	top             int
	timeout         time.Duration
}

type runtimeValues struct {
	dbDSN           string
	objectProvider  string
	objectEndpoint  string
	objectAccessKey string
	objectSecretKey string
	objectRegion    string
	objectSecure    string
	gcsProjectID    string
}

type deviceRecord struct {
	deviceID         string
	providerDeviceID string
	model            string
	productName      string
	metadata         map[string]any
	capabilities     map[string]any
}

type manifestObject struct {
	bucket      string
	key         string
	recordCount int
}

type fieldStats struct {
	count       int
	nonZero     int
	transitions int
	min         float64
	max         float64
	last        string
	lastSeen    string
	hasNumber   bool
	distinct    map[string]int
}

type report struct {
	objectsMatched    int
	objectsProcessed  int
	framesDecoded     int
	framesMatched     int
	payloadSearchHits int
	first             time.Time
	last              time.Time
	payloadTypes      map[string]int
	sources           map[string]int
	sourceKinds       map[string]int
	fields            map[string]*fieldStats
	fieldSearchHits   map[string]int
	parseFailures     int
}

func main() {
	cfg := parseFlags()
	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.provider, "provider", envOr("PULSE_ARCHIVE_PROVIDER", "pecron"), "Provider filter, e.g. pecron, ecoflow, anker_solix, or empty for all")
	flag.StringVar(&cfg.family, "family", "", "Comma-separated model/product family filters, e.g. E1000LFP,DPU")
	flag.StringVar(&cfg.search, "search", "", "Comma-separated field/payload substrings to count, e.g. fan,cool,heat")
	flag.StringVar(&cfg.payloadType, "payload-type", "", "Optional payload type filter, e.g. provider.params.normalized or ecoflow.mqtt.raw")
	flag.Float64Var(&cfg.hours, "hours", 12, "Lookback window in hours; ignored when --from is set")
	flag.StringVar(&cfg.from, "from", "", "UTC/RFC3339 window start, e.g. 2026-05-21T00:00:00Z")
	flag.StringVar(&cfg.to, "to", "", "UTC/RFC3339 window end; defaults to now")
	flag.StringVar(&cfg.dbDSN, "db-dsn", envOr("CONTROL_PLANE_DB_DSN", envOr("ARCHIVE_MANIFEST_DB_DSN", "")), "Postgres DSN; omit to use env or --load-runtime")
	flag.StringVar(&cfg.objectProvider, "object-provider", envOr("ARCHIVE_OBJECT_PROVIDER", ""), "Object provider: gcs or minio")
	flag.StringVar(&cfg.objectEndpoint, "object-endpoint", envOr("ARCHIVE_OBJECT_ENDPOINT", ""), "MinIO endpoint when --object-provider=minio")
	flag.StringVar(&cfg.objectAccessKey, "object-access-key", envOr("ARCHIVE_OBJECT_ACCESS_KEY", ""), "MinIO access key")
	flag.StringVar(&cfg.objectSecretKey, "object-secret-key", envOr("ARCHIVE_OBJECT_SECRET_KEY", ""), "MinIO secret key")
	flag.StringVar(&cfg.objectRegion, "object-region", envOr("ARCHIVE_OBJECT_REGION", ""), "Object region")
	flag.StringVar(&cfg.objectSecure, "object-secure", envOr("ARCHIVE_OBJECT_SECURE", ""), "Object TLS flag for MinIO")
	flag.StringVar(&cfg.gcsProjectID, "gcs-project-id", envOr("ARCHIVE_OBJECT_GCS_PROJECT_ID", envOr("GOOGLE_CLOUD_PROJECT", "")), "GCS project id")
	flag.StringVar(&cfg.preset, "preset", "", "Preset: cloud-gcs, local-minio, or local-edge-cloud-archive")
	flag.StringVar(&cfg.kubeContext, "kube-context", "", "Optional kube context for loading runtime secret/config")
	flag.StringVar(&cfg.kubeNamespace, "kube-namespace", defaultNamespace, "Kubernetes namespace for runtime secret/config")
	flag.StringVar(&cfg.runtimeSecret, "runtime-secret", defaultRuntimeSecret, "Runtime secret name")
	flag.StringVar(&cfg.runtimeConfig, "runtime-config", defaultRuntimeConfig, "Runtime configmap name")
	flag.BoolVar(&cfg.loadRuntime, "load-runtime", false, "Load DSN/object env from Kubernetes runtime secret/config")
	flag.BoolVar(&cfg.rewriteDocker, "rewrite-host-docker", true, "Rewrite host.docker.internal to 127.0.0.1 in loaded DSNs for host-side runs")
	flag.IntVar(&cfg.maxObjects, "max-objects", 0, "Optional manifest object cap for quick sampling")
	flag.IntVar(&cfg.top, "top", 80, "Maximum field rows to print")
	flag.DurationVar(&cfg.timeout, "timeout", 2*time.Minute, "Overall command timeout")
	flag.Parse()
	return cfg
}

func run(cfg config) error {
	applyPreset(&cfg)
	if cfg.loadRuntime || cfg.kubeContext != "" {
		values, err := loadRuntimeValues(cfg)
		if err != nil {
			return err
		}
		mergeRuntimeValues(&cfg, values)
	}
	applyPreset(&cfg)
	if cfg.rewriteDocker {
		cfg.dbDSN = strings.ReplaceAll(cfg.dbDSN, "host=host.docker.internal", "host=127.0.0.1")
		cfg.dbDSN = strings.ReplaceAll(cfg.dbDSN, "@host.docker.internal:", "@127.0.0.1:")
	}
	if strings.TrimSpace(cfg.dbDSN) == "" {
		return fmt.Errorf("CONTROL_PLANE_DB_DSN/ARCHIVE_MANIFEST_DB_DSN or --db-dsn is required")
	}
	from, to, err := resolveWindow(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.dbDSN)
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	defer pool.Close()

	familyTerms := splitTerms(cfg.family)
	devices, err := loadProviderDevices(ctx, pool, cfg.provider, familyTerms)
	if err != nil {
		return err
	}
	deviceIDs := make([]string, 0, len(devices))
	for _, device := range devices {
		if strings.TrimSpace(device.deviceID) != "" {
			deviceIDs = append(deviceIDs, device.deviceID)
		}
	}
	objects, err := loadManifestObjects(ctx, pool, cfg.provider, deviceIDs, from, to, cfg.maxObjects)
	if err != nil {
		return err
	}
	reader, err := replaycli.NewObjectReader(objectReaderConfig(cfg))
	if err != nil {
		return fmt.Errorf("open archive object reader: %w", err)
	}
	defer func() { _ = reader.Close() }()

	rep := inspectArchive(ctx, reader, objects, cfg, from, to, makeSet(deviceIDs), splitTerms(cfg.search))
	printReport(cfg, from, to, devices, objects, rep)
	return nil
}

func applyPreset(cfg *config) {
	switch strings.ToLower(strings.TrimSpace(cfg.preset)) {
	case "", "none":
		return
	case "cloud-gcs", "local-edge-cloud-archive":
		if cfg.objectProvider == "" {
			cfg.objectProvider = "gcs"
		}
		if cfg.gcsProjectID == "" {
			cfg.gcsProjectID = defaultCloudProject
		}
		if cfg.objectRegion == "" {
			cfg.objectRegion = "us-east1"
		}
	case "local-minio":
		if cfg.objectProvider == "" {
			cfg.objectProvider = "minio"
		}
		if cfg.objectEndpoint == "" {
			cfg.objectEndpoint = "127.0.0.1:9000"
		}
		if cfg.objectAccessKey == "" {
			cfg.objectAccessKey = "minio"
		}
		if cfg.objectSecretKey == "" {
			cfg.objectSecretKey = "minio123"
		}
		if cfg.objectRegion == "" {
			cfg.objectRegion = "us-east-1"
		}
		if cfg.objectSecure == "" {
			cfg.objectSecure = "false"
		}
	default:
		fmt.Fprintf(os.Stderr, "warning: unknown preset %q; using explicit/env values\n", cfg.preset)
	}
}

func objectReaderConfig(cfg config) replaycli.ObjectReaderConfig {
	provider := strings.ToLower(strings.TrimSpace(cfg.objectProvider))
	if provider == "" {
		if strings.TrimSpace(cfg.gcsProjectID) != "" {
			provider = "gcs"
		} else {
			provider = "minio"
		}
	}
	region := strings.TrimSpace(cfg.objectRegion)
	if region == "" {
		if provider == "gcs" {
			region = "us-east1"
		} else {
			region = "us-east-1"
		}
	}
	return replaycli.ObjectReaderConfig{
		Provider:        replaycli.ObjectProvider(provider),
		Endpoint:        cfg.objectEndpoint,
		AccessKeyID:     cfg.objectAccessKey,
		SecretAccessKey: cfg.objectSecretKey,
		Region:          region,
		Secure:          parseBool(cfg.objectSecure),
		GCSProjectID:    cfg.gcsProjectID,
	}
}

func loadProviderDevices(ctx context.Context, pool *pgxpool.Pool, provider string, familyTerms []string) ([]deviceRecord, error) {
	rows, err := pool.Query(ctx, `
SELECT device_id::text, provider_device_id, COALESCE(model, ''), COALESCE(product_name, ''), metadata, capabilities
FROM provider_devices
WHERE ($1::text = '' OR provider = $1::text)
ORDER BY provider, model, product_name, updated_at DESC
`, strings.ToLower(strings.TrimSpace(provider)))
	if err != nil {
		return nil, fmt.Errorf("query provider devices: %w", err)
	}
	defer rows.Close()

	out := make([]deviceRecord, 0, 8)
	for rows.Next() {
		var rec deviceRecord
		var metadataRaw []byte
		var capabilitiesRaw []byte
		if err := rows.Scan(&rec.deviceID, &rec.providerDeviceID, &rec.model, &rec.productName, &metadataRaw, &capabilitiesRaw); err != nil {
			return nil, fmt.Errorf("scan provider device: %w", err)
		}
		rec.metadata = decodeJSONMap(metadataRaw)
		rec.capabilities = decodeJSONMap(capabilitiesRaw)
		if len(familyTerms) > 0 && !matchesFamily(rec, familyTerms) {
			continue
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider devices: %w", err)
	}
	return out, nil
}

func matchesFamily(rec deviceRecord, terms []string) bool {
	haystackParts := []string{
		rec.model,
		rec.productName,
		fmt.Sprint(rec.metadata["product_key"]),
		fmt.Sprint(rec.metadata["provider"]),
		fmt.Sprint(rec.capabilities["device_family"]),
	}
	rawMetadata, _ := json.Marshal(rec.metadata)
	rawCapabilities, _ := json.Marshal(rec.capabilities)
	haystackParts = append(haystackParts, string(rawMetadata), string(rawCapabilities))
	haystack := strings.ToLower(strings.Join(haystackParts, " "))
	for _, term := range terms {
		if strings.Contains(haystack, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func loadManifestObjects(ctx context.Context, pool *pgxpool.Pool, provider string, deviceIDs []string, from time.Time, to time.Time, maxObjects int) ([]manifestObject, error) {
	sql := `
SELECT object_bucket, object_key, record_count
FROM archive_object_manifest
WHERE ($1::text = '' OR provider = $1::text)
  AND ts_max_unix_ms >= $2
  AND ts_min_unix_ms <= $3
  AND (COALESCE(cardinality($4::text[]), 0) = 0 OR device_ids && $4::text[])
ORDER BY partition_hour ASC, shard ASC, object_key ASC
`
	args := []any{strings.ToLower(strings.TrimSpace(provider)), from.UnixMilli(), to.UnixMilli(), deviceIDs}
	if maxObjects > 0 {
		sql += " LIMIT $5"
		args = append(args, maxObjects)
	}
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query archive manifest: %w", err)
	}
	defer rows.Close()
	out := make([]manifestObject, 0, 128)
	for rows.Next() {
		var object manifestObject
		if err := rows.Scan(&object.bucket, &object.key, &object.recordCount); err != nil {
			return nil, fmt.Errorf("scan manifest object: %w", err)
		}
		out = append(out, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate manifest objects: %w", err)
	}
	return out, nil
}

func inspectArchive(ctx context.Context, reader replaycli.ObjectReader, objects []manifestObject, cfg config, from time.Time, to time.Time, deviceSet map[string]struct{}, searchTerms []string) report {
	rep := report{
		payloadTypes:    map[string]int{},
		sources:         map[string]int{},
		sourceKinds:     map[string]int{},
		fields:          map[string]*fieldStats{},
		fieldSearchHits: map[string]int{},
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.provider))
	payloadType := strings.TrimSpace(cfg.payloadType)
	for _, object := range objects {
		if err := ctx.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: stopping early: %v\n", err)
			break
		}
		body, err := reader.ReadObject(ctx, object.bucket, object.key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to read one archive object: %v\n", err)
			continue
		}
		rep.objectsProcessed++
		frames, err := replaycli.DecodeEnvelopeFrames(body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to decode one archive object: %v\n", err)
			continue
		}
		for _, frame := range frames {
			rep.framesDecoded++
			var env envelopev1.TelemetryEnvelope
			if err := proto.Unmarshal(frame, &env); err != nil {
				rep.parseFailures++
				continue
			}
			if !matchesEnvelope(&env, provider, deviceSet) {
				continue
			}
			if payloadType != "" && env.GetPayloadType() != payloadType {
				continue
			}
			at := envelopeTime(&env)
			if at.Before(from) || at.After(to) {
				continue
			}
			rep.framesMatched++
			if rep.first.IsZero() || at.Before(rep.first) {
				rep.first = at
			}
			if rep.last.IsZero() || at.After(rep.last) {
				rep.last = at
			}
			rep.payloadTypes[blankLabel(env.GetPayloadType())]++
			rep.sources[blankLabel(env.GetSource())]++
			rep.sourceKinds[env.GetSourceKind().String()]++
			payload := env.GetPayload()
			if containsAny(string(payload), searchTerms) {
				rep.payloadSearchHits++
			}
			fields, ok := flattenPayloadFields(payload)
			if !ok {
				rep.parseFailures++
				continue
			}
			for key, value := range fields {
				updateStats(rep.fields, key, value)
				if containsAny(key, searchTerms) {
					rep.fieldSearchHits[key]++
				}
			}
		}
	}
	return rep
}

func matchesEnvelope(env *envelopev1.TelemetryEnvelope, provider string, deviceSet map[string]struct{}) bool {
	if len(deviceSet) > 0 {
		if _, ok := deviceSet[strings.TrimSpace(env.GetDeviceId())]; !ok {
			return false
		}
	}
	if provider == "" {
		return true
	}
	if strings.EqualFold(env.GetLabels()["provider"], provider) {
		return true
	}
	if provider == "pecron" && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(env.GetEcoflowSn())), "P11") {
		return true
	}
	return false
}

func flattenPayloadFields(payload []byte) (map[string]any, bool) {
	var root any
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		return nil, false
	}
	rootMap, _ := root.(map[string]any)
	fields := map[string]any{}
	if params, ok := rootMap["params"]; ok {
		flattenJSON("params", params, fields)
		return fields, true
	}
	flattenJSON("", root, fields)
	return fields, true
}

func flattenJSON(prefix string, value any, out map[string]any) {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			flattenJSON(next, v[key], out)
		}
	case []any:
		for idx, item := range v {
			next := fmt.Sprintf("%s[%d]", prefix, idx)
			flattenJSON(next, item, out)
		}
	default:
		if prefix != "" {
			out[prefix] = value
		}
	}
}

func updateStats(stats map[string]*fieldStats, key string, value any) {
	stat := stats[key]
	if stat == nil {
		stat = &fieldStats{min: math.Inf(1), max: math.Inf(-1), distinct: map[string]int{}}
		stats[key] = stat
	}
	stat.count++
	valueString := summarizeValue(value)
	stat.last = valueString
	if stat.lastSeen != "" && stat.lastSeen != valueString {
		stat.transitions++
	}
	stat.lastSeen = valueString
	if len(stat.distinct) < 16 {
		stat.distinct[valueString]++
	}
	if number, ok := numberValue(value); ok {
		stat.hasNumber = true
		if number != 0 {
			stat.nonZero++
		}
		if number < stat.min {
			stat.min = number
		}
		if number > stat.max {
			stat.max = number
		}
	}
}

func printReport(cfg config, from time.Time, to time.Time, devices []deviceRecord, objects []manifestObject, rep report) {
	fmt.Printf("window_utc=%s..%s\n", from.Format(time.RFC3339), to.Format(time.RFC3339))
	fmt.Printf("provider=%s family=%s payload_type_filter=%s\n", blankLabel(cfg.provider), blankLabel(cfg.family), blankLabel(cfg.payloadType))
	fmt.Printf("matched_devices=%d redaction=ids-hidden\n", len(devices))
	fmt.Printf("archive_objects_matched=%d processed=%d\n", len(objects), rep.objectsProcessed)
	fmt.Printf("frames_decoded=%d matched=%d first=%s last=%s parse_failures=%d\n", rep.framesDecoded, rep.framesMatched, formatTime(rep.first), formatTime(rep.last), rep.parseFailures)
	fmt.Printf("payload_types=%s\n", formatIntMap(rep.payloadTypes))
	fmt.Printf("sources=%s\n", formatIntMap(rep.sources))
	fmt.Printf("source_kinds=%s\n", formatIntMap(rep.sourceKinds))
	searchTerms := splitTerms(cfg.search)
	if len(searchTerms) > 0 {
		fmt.Printf("search_terms=%s payload_hits=%d field_hits=%s\n", strings.Join(searchTerms, ","), rep.payloadSearchHits, formatIntMap(rep.fieldSearchHits))
	}
	fmt.Printf("field_count=%d\n", len(rep.fields))
	for _, row := range selectedFieldRows(rep.fields, rep.fieldSearchHits, cfg.top) {
		fmt.Printf(
			"field[%s]=count=%d nonzero=%d transitions=%d min=%s max=%s last=%s distinct=%s\n",
			row.key,
			row.stat.count,
			row.stat.nonZero,
			row.stat.transitions,
			formatFloat(row.stat.min, row.stat.hasNumber),
			formatFloat(row.stat.max, row.stat.hasNumber),
			row.stat.last,
			formatDistinct(row.stat.distinct),
		)
	}
}

type fieldRow struct {
	key  string
	stat *fieldStats
}

func selectedFieldRows(stats map[string]*fieldStats, searchHits map[string]int, top int) []fieldRow {
	rows := make([]fieldRow, 0, len(stats))
	for key, stat := range stats {
		rows = append(rows, fieldRow{key: key, stat: stat})
	}
	sort.Slice(rows, func(i, j int) bool {
		leftSearch := searchHits[rows[i].key] > 0
		rightSearch := searchHits[rows[j].key] > 0
		if leftSearch != rightSearch {
			return leftSearch
		}
		if rows[i].stat.transitions != rows[j].stat.transitions {
			return rows[i].stat.transitions > rows[j].stat.transitions
		}
		if rows[i].stat.nonZero != rows[j].stat.nonZero {
			return rows[i].stat.nonZero > rows[j].stat.nonZero
		}
		if rows[i].stat.count != rows[j].stat.count {
			return rows[i].stat.count > rows[j].stat.count
		}
		return rows[i].key < rows[j].key
	})
	if top > 0 && len(rows) > top {
		return rows[:top]
	}
	return rows
}

func loadRuntimeValues(cfg config) (runtimeValues, error) {
	values := runtimeValues{}
	secret, err := kubectlJSON(cfg, "secret", cfg.runtimeSecret)
	if err != nil {
		return values, err
	}
	configMap, err := kubectlJSON(cfg, "configmap", cfg.runtimeConfig)
	if err != nil {
		return values, err
	}
	secretData := map[string]string{}
	if raw, ok := secret["data"].(map[string]any); ok {
		for key, value := range raw {
			decoded, err := base64.StdEncoding.DecodeString(fmt.Sprint(value))
			if err == nil {
				secretData[key] = string(decoded)
			}
		}
	}
	configData := map[string]string{}
	if raw, ok := configMap["data"].(map[string]any); ok {
		for key, value := range raw {
			configData[key] = fmt.Sprint(value)
		}
	}
	values.dbDSN = firstNonEmpty(secretData["CONTROL_PLANE_DB_DSN"], secretData["ARCHIVE_MANIFEST_DB_DSN"])
	values.objectAccessKey = secretData["ARCHIVE_OBJECT_ACCESS_KEY"]
	values.objectSecretKey = secretData["ARCHIVE_OBJECT_SECRET_KEY"]
	values.objectProvider = configData["ARCHIVE_OBJECT_PROVIDER"]
	values.objectEndpoint = configData["ARCHIVE_OBJECT_ENDPOINT"]
	values.objectRegion = configData["ARCHIVE_OBJECT_REGION"]
	values.objectSecure = configData["ARCHIVE_OBJECT_SECURE"]
	values.gcsProjectID = configData["ARCHIVE_OBJECT_GCS_PROJECT_ID"]
	return values, nil
}

func kubectlJSON(cfg config, kind string, name string) (map[string]any, error) {
	args := []string{}
	if strings.TrimSpace(cfg.kubeContext) != "" {
		args = append(args, "--context", strings.TrimSpace(cfg.kubeContext))
	}
	args = append(args, "-n", strings.TrimSpace(cfg.kubeNamespace), "get", kind, strings.TrimSpace(name), "-o", "json")
	cmd := exec.Command("kubectl", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl get %s/%s failed: %w", kind, name, err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		return nil, fmt.Errorf("parse kubectl %s/%s JSON: %w", kind, name, err)
	}
	return decoded, nil
}

func mergeRuntimeValues(cfg *config, values runtimeValues) {
	if cfg.dbDSN == "" {
		cfg.dbDSN = values.dbDSN
	}
	if cfg.objectProvider == "" {
		cfg.objectProvider = values.objectProvider
	}
	if cfg.objectEndpoint == "" {
		cfg.objectEndpoint = values.objectEndpoint
	}
	if cfg.objectAccessKey == "" {
		cfg.objectAccessKey = values.objectAccessKey
	}
	if cfg.objectSecretKey == "" {
		cfg.objectSecretKey = values.objectSecretKey
	}
	if cfg.objectRegion == "" {
		cfg.objectRegion = values.objectRegion
	}
	if cfg.objectSecure == "" {
		cfg.objectSecure = values.objectSecure
	}
	if cfg.gcsProjectID == "" {
		cfg.gcsProjectID = values.gcsProjectID
	}
}

func resolveWindow(cfg config) (time.Time, time.Time, error) {
	to := time.Now().UTC()
	if strings.TrimSpace(cfg.to) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(cfg.to))
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("parse --to: %w", err)
		}
		to = parsed.UTC()
	}
	var from time.Time
	if strings.TrimSpace(cfg.from) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(cfg.from))
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("parse --from: %w", err)
		}
		from = parsed.UTC()
	} else {
		if cfg.hours <= 0 {
			return time.Time{}, time.Time{}, fmt.Errorf("--hours must be > 0")
		}
		from = to.Add(-time.Duration(cfg.hours * float64(time.Hour)))
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("window end must be after start")
	}
	return from, to, nil
}

func envelopeTime(env *envelopev1.TelemetryEnvelope) time.Time {
	if ts := env.GetIngestedTimeUnixMs(); ts > 0 {
		return time.UnixMilli(ts).UTC()
	}
	if ts := env.GetObservedTimeUnixMs(); ts > 0 {
		return time.UnixMilli(ts).UTC()
	}
	return time.Time{}
}

func decodeJSONMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func splitTerms(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		term := strings.ToLower(strings.TrimSpace(part))
		if term == "" {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		out = append(out, term)
	}
	sort.Strings(out)
	return out
}

func containsAny(value string, terms []string) bool {
	if len(terms) == 0 {
		return false
	}
	lower := strings.ToLower(value)
	for _, term := range terms {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func makeSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = struct{}{}
		}
	}
	return out
}

func numberValue(value any) (float64, bool) {
	switch v := value.(type) {
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func summarizeValue(value any) string {
	if number, ok := numberValue(value); ok {
		return formatFloat(number, true)
	}
	switch v := value.(type) {
	case bool:
		if v {
			return "true"
		}
		return "false"
	case string:
		trimmed := strings.TrimSpace(v)
		if len(trimmed) > 48 {
			return trimmed[:48] + "..."
		}
		return trimmed
	default:
		return fmt.Sprintf("%T", value)
	}
}

func formatFloat(value float64, ok bool) string {
	if !ok || math.IsInf(value, 0) || math.IsNaN(value) {
		return "n/a"
	}
	return fmt.Sprintf("%.3f", value)
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "none"
	}
	return value.UTC().Format(time.RFC3339)
}

func formatIntMap(values map[string]int) string {
	if len(values) == 0 {
		return "(none)"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", blankLabel(key), values[key]))
	}
	return strings.Join(parts, ",")
}

func formatDistinct(values map[string]int) string {
	if len(values) == 0 {
		return "(none)"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > 8 {
		keys = keys[:8]
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", blankLabel(key), values[key]))
	}
	return strings.Join(parts, ",")
}

func blankLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "(blank)"
	}
	return value
}

func envOr(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes", "y", "on":
		return true
	default:
		return false
	}
}
