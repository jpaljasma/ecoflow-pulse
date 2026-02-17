package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultCSVPath = "/Users/jpaljasma/Downloads/solar_panel_specs_with_ecoflow_compat_cold_voc_and_safety_margins_v13.csv"
	defaultOutJSON = "data/solar_panels/solar_panel_specs_v13.json"
	defaultSummary = "data/solar_panels/solar_panel_specs_v13.summary.json"
	defaultIndex   = "data/solar_panels/solar_panel_specs_v13.index.json"
)

var (
	nonAlphaNum      = regexp.MustCompile(`[^a-z0-9_]+`)
	compatSeriesMin  = regexp.MustCompile(`(?i)needs\s*≥?\s*(\d+)\s*s`)
	compatSeriesMax  = regexp.MustCompile(`(?i)\(max\s*(\d+)\s*s\)`)
	deviceSpecSuffix = regexp.MustCompile(`(?i)\s+\d+\s*[–-]\s*\d+\s*v[^\s]*\s*$`)
)

type dataset struct {
	SourceCSV       string            `json:"source_csv"`
	GeneratedAtUTC  string            `json:"generated_at_utc"`
	RowCount        int               `json:"row_count"`
	ColumnCount     int               `json:"column_count"`
	ColumnsOriginal []string          `json:"columns_original"`
	ColumnsNorm     map[string]string `json:"columns_normalized"`
	Summary         datasetSummary    `json:"summary"`
	Records         []map[string]any  `json:"records"`
}

type datasetSummary struct {
	Rows                              int            `json:"rows"`
	Columns                           int            `json:"columns"`
	MissingByColumn                   map[string]int `json:"missing_by_column"`
	NonEmptyByColumn                  map[string]int `json:"non_empty_by_column"`
	DistinctValuesByColumn            map[string]int `json:"distinct_values_by_column"`
	BrandDistribution                 map[string]int `json:"brand_distribution"`
	TypeDistribution                  map[string]int `json:"type_distribution"`
	VocTempCoeffMissingTrueCount      int            `json:"voc_temp_coeff_missing_true_count"`
	EcoflowCompatibilityNonEmptyCount int            `json:"ecoflow_compatibility_non_empty_count"`
}

type panelIndex struct {
	SourceCSV      string                     `json:"source_csv"`
	GeneratedAtUTC string                     `json:"generated_at_utc"`
	RowCount       int                        `json:"row_count"`
	PanelCount     int                        `json:"panel_count"`
	ByPanelKey     map[string]panelIndexEntry `json:"by_panel_key"`
	ByDeviceTag    map[string][]string        `json:"by_device_tag"`
	DeviceLabels   map[string]string          `json:"device_labels"`
}

type panelIndexEntry struct {
	ID                string                               `json:"id"`
	Brand             string                               `json:"brand"`
	Model             string                               `json:"model"`
	Type              string                               `json:"type,omitempty"`
	PmaxSTCW          float64                              `json:"pmax_stc_w,omitempty"`
	VocV              float64                              `json:"voc_v,omitempty"`
	VmpV              float64                              `json:"vmp_v,omitempty"`
	ImpA              float64                              `json:"imp_a,omitempty"`
	IscA              float64                              `json:"isc_a,omitempty"`
	CompatibilityTags []string                             `json:"compatibility_tags"`
	Compatibility     map[string]panelCompatibilitySummary `json:"compatibility"`
}

type panelCompatibilitySummary struct {
	Label             string `json:"label"`
	Status            string `json:"status"`
	MinSeries         int    `json:"min_series,omitempty"`
	MaxSeries         int    `json:"max_series,omitempty"`
	CurrentClipLikely bool   `json:"current_clip_likely,omitempty"`
}

func main() {
	var (
		csvPath     string
		outPath     string
		summaryPath string
		indexPath   string
	)

	flag.StringVar(&csvPath, "csv", defaultCSVPath, "input panel CSV path")
	flag.StringVar(&outPath, "out", defaultOutJSON, "output normalized JSON path")
	flag.StringVar(&summaryPath, "summary-out", defaultSummary, "output summary JSON path")
	flag.StringVar(&indexPath, "index-out", defaultIndex, "output compact index JSON path")
	flag.Parse()

	data, err := importCSV(csvPath)
	if err != nil {
		fatalf("import csv: %v", err)
	}

	if err := writeJSON(outPath, data); err != nil {
		fatalf("write output json: %v", err)
	}
	if err := writeJSON(summaryPath, data.Summary); err != nil {
		fatalf("write summary json: %v", err)
	}
	index := buildPanelIndex(data)
	if err := writeJSON(indexPath, index); err != nil {
		fatalf("write compact index json: %v", err)
	}

	fmt.Printf("source: %s\n", csvPath)
	fmt.Printf("rows: %d, columns: %d\n", data.RowCount, data.ColumnCount)
	fmt.Printf("output: %s\n", outPath)
	fmt.Printf("summary: %s\n", summaryPath)
	fmt.Printf("index: %s\n", indexPath)
}

func importCSV(path string) (*dataset, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("csv has no data rows")
	}

	header := rows[0]
	normalized := make([]string, len(header))
	columnsNorm := make(map[string]string, len(header))
	for i, col := range header {
		key := normalizeColumn(col)
		if key == "" {
			key = fmt.Sprintf("col_%d", i+1)
		}
		normalized[i] = key
		columnsNorm[col] = key
	}

	missing := map[string]int{}
	nonEmpty := map[string]int{}
	distinct := map[string]map[string]struct{}{}
	for _, key := range normalized {
		distinct[key] = map[string]struct{}{}
	}

	brandDist := map[string]int{}
	typeDist := map[string]int{}
	vocTempMissingTrue := 0
	compatNonEmpty := 0
	records := make([]map[string]any, 0, len(rows)-1)

	for rowIdx, row := range rows[1:] {
		record := make(map[string]any, len(normalized)+4)
		for i, key := range normalized {
			raw := ""
			if i < len(row) {
				raw = strings.TrimSpace(row[i])
			}
			if raw == "" {
				missing[key]++
				record[key] = nil
				continue
			}

			nonEmpty[key]++
			distinct[key][raw] = struct{}{}
			record[key] = coerceValue(raw)
		}

		record["source_row"] = rowIdx + 2
		record["id"] = makeRecordID(record, rowIdx)

		compatKey := normalizeColumn("EcoFlow_compatibility")
		if compatRaw, ok := record[compatKey].(string); ok && strings.TrimSpace(compatRaw) != "" {
			compatNonEmpty++
			record["ecoflow_compatibility_entries"] = splitCompatibility(compatRaw)
		} else {
			record["ecoflow_compatibility_entries"] = []string{}
		}

		if val, ok := record[normalizeColumn("Voc_temp_coeff_missing")].(bool); ok && val {
			vocTempMissingTrue++
		}
		if brand, ok := record[normalizeColumn("Brand")].(string); ok && brand != "" {
			brandDist[brand]++
		}
		if typ, ok := record[normalizeColumn("Type")].(string); ok && typ != "" {
			typeDist[typ]++
		}

		records = append(records, record)
	}

	distinctCounts := map[string]int{}
	for key, values := range distinct {
		distinctCounts[key] = len(values)
	}

	sort.Slice(records, func(i, j int) bool {
		leftSN, _ := records[i][normalizeColumn("Brand")].(string)
		rightSN, _ := records[j][normalizeColumn("Brand")].(string)
		if leftSN != rightSN {
			return leftSN < rightSN
		}
		leftModel, _ := records[i][normalizeColumn("Model")].(string)
		rightModel, _ := records[j][normalizeColumn("Model")].(string)
		return leftModel < rightModel
	})

	result := &dataset{
		SourceCSV:       path,
		GeneratedAtUTC:  time.Now().UTC().Format(time.RFC3339),
		RowCount:        len(records),
		ColumnCount:     len(header),
		ColumnsOriginal: header,
		ColumnsNorm:     columnsNorm,
		Summary: datasetSummary{
			Rows:                              len(records),
			Columns:                           len(header),
			MissingByColumn:                   missing,
			NonEmptyByColumn:                  nonEmpty,
			DistinctValuesByColumn:            distinctCounts,
			BrandDistribution:                 brandDist,
			TypeDistribution:                  typeDist,
			VocTempCoeffMissingTrueCount:      vocTempMissingTrue,
			EcoflowCompatibilityNonEmptyCount: compatNonEmpty,
		},
		Records: records,
	}
	return result, nil
}

func buildPanelIndex(data *dataset) *panelIndex {
	brandKey := normalizeColumn("Brand")
	modelKey := normalizeColumn("Model")
	typeKey := normalizeColumn("Type")
	pmaxKey := normalizeColumn("Pmax_STC_W")
	vocKey := normalizeColumn("Voc_V")
	vmpKey := normalizeColumn("Vmp_V")
	impKey := normalizeColumn("Imp_A")
	iscKey := normalizeColumn("Isc_A")

	byPanelKey := make(map[string]panelIndexEntry, len(data.Records))
	byDeviceSet := map[string]map[string]struct{}{}
	deviceLabels := map[string]string{}

	for _, record := range data.Records {
		id := asString(record["id"])
		brand := asString(record[brandKey])
		model := asString(record[modelKey])
		panelKey := makePanelKey(brand, model, id, asInt(record["source_row"]), byPanelKey)

		entry := panelIndexEntry{
			ID:            id,
			Brand:         brand,
			Model:         model,
			Type:          asString(record[typeKey]),
			PmaxSTCW:      asFloat64(record[pmaxKey]),
			VocV:          asFloat64(record[vocKey]),
			VmpV:          asFloat64(record[vmpKey]),
			ImpA:          asFloat64(record[impKey]),
			IscA:          asFloat64(record[iscKey]),
			Compatibility: map[string]panelCompatibilitySummary{},
		}

		tagSet := map[string]struct{}{}
		for _, raw := range asStringSlice(record["ecoflow_compatibility_entries"]) {
			tag, summary := parseCompatibilityEntry(raw)
			if tag == "" {
				continue
			}

			if existing, ok := entry.Compatibility[tag]; ok {
				entry.Compatibility[tag] = mergeCompatibility(existing, summary)
			} else {
				entry.Compatibility[tag] = summary
			}

			tagSet[tag] = struct{}{}
			if _, ok := byDeviceSet[tag]; !ok {
				byDeviceSet[tag] = map[string]struct{}{}
			}
			byDeviceSet[tag][panelKey] = struct{}{}
			if deviceLabels[tag] == "" {
				deviceLabels[tag] = summary.Label
			}
		}

		entry.CompatibilityTags = sortedSet(tagSet)
		byPanelKey[panelKey] = entry
	}

	byDeviceTag := map[string][]string{}
	for tag, keySet := range byDeviceSet {
		keys := sortedSet(keySet)
		byDeviceTag[tag] = keys
	}

	return &panelIndex{
		SourceCSV:      data.SourceCSV,
		GeneratedAtUTC: data.GeneratedAtUTC,
		RowCount:       data.RowCount,
		PanelCount:     len(byPanelKey),
		ByPanelKey:     byPanelKey,
		ByDeviceTag:    byDeviceTag,
		DeviceLabels:   deviceLabels,
	}
}

func parseCompatibilityEntry(raw string) (string, panelCompatibilitySummary) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", panelCompatibilitySummary{}
	}
	parts := strings.SplitN(raw, ":", 2)
	label := strings.TrimSpace(parts[0])
	if label == "" || strings.EqualFold(label, "note") {
		return "", panelCompatibilitySummary{}
	}
	statusText := ""
	if len(parts) == 2 {
		statusText = strings.TrimSpace(parts[1])
	}
	status := classifyCompatibilityStatus(statusText)
	summary := panelCompatibilitySummary{
		Label:  label,
		Status: status,
	}
	if match := compatSeriesMin.FindStringSubmatch(statusText); len(match) == 2 {
		if parsed, err := strconv.Atoi(match[1]); err == nil {
			summary.MinSeries = parsed
		}
	}
	if match := compatSeriesMax.FindStringSubmatch(statusText); len(match) == 2 {
		if parsed, err := strconv.Atoi(match[1]); err == nil {
			summary.MaxSeries = parsed
		}
	}
	if strings.Contains(strings.ToLower(statusText), "current clip likely") {
		summary.CurrentClipLikely = true
	}
	return makeDeviceTag(label), summary
}

func mergeCompatibility(existing, next panelCompatibilitySummary) panelCompatibilitySummary {
	if existing.Label == "" {
		existing.Label = next.Label
	}
	if existing.Status == "unknown" && next.Status != "unknown" {
		existing.Status = next.Status
	}
	if existing.MinSeries == 0 || (next.MinSeries > 0 && next.MinSeries < existing.MinSeries) {
		existing.MinSeries = next.MinSeries
	}
	if next.MaxSeries > existing.MaxSeries {
		existing.MaxSeries = next.MaxSeries
	}
	existing.CurrentClipLikely = existing.CurrentClipLikely || next.CurrentClipLikely
	return existing
}

func classifyCompatibilityStatus(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "unknown"
	}
	switch {
	case strings.Contains(value, "yes"):
		return "yes"
	case strings.Contains(value, "no"):
		return "no"
	case strings.Contains(value, "needs"):
		return "needs_series"
	default:
		return "unknown"
	}
}

func makeDeviceTag(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	label = strings.NewReplacer("‑", "-", "–", "-", "—", "-").Replace(label)
	label = deviceSpecSuffix.ReplaceAllString(label, "")
	label = strings.TrimSpace(label)
	return normalizeColumn(label)
}

func makePanelKey(brand, model, fallbackID string, sourceRow int, existing map[string]panelIndexEntry) string {
	base := normalizeColumn(strings.TrimSpace(brand) + "_" + strings.TrimSpace(model))
	if base == "" {
		base = normalizeColumn(fallbackID)
	}
	if base == "" {
		base = fmt.Sprintf("row_%d", sourceRow)
	}
	key := base
	for i := 2; ; i++ {
		if _, ok := existing[key]; !ok {
			return key
		}
		key = fmt.Sprintf("%s_%d", base, i)
	}
}

func sortedSet(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func normalizeColumn(col string) string {
	s := strings.TrimSpace(strings.ToLower(col))
	if s == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"%/c", "_pct_per_c",
		"%", "_pct",
		"/", "_",
		"(", "_",
		")", "_",
		"-", "_",
		"–", "_",
		"—", "_",
		"+", "_plus_",
		".", "_",
		" ", "_",
	)
	s = replacer.Replace(s)
	s = nonAlphaNum.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return s
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func asInt(value any) int {
	minInt, maxInt := intBounds()
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		if typed < minInt || typed > maxInt {
			return 0
		}
		return int(typed)
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0
		}
		truncated := math.Trunc(typed)
		if truncated < float64(minInt) || truncated > float64(maxInt) {
			return 0
		}
		return int(typed)
	default:
		return 0
	}
}

func intBounds() (int64, int64) {
	maxInt := int64(^uint(0) >> 1)
	minInt := -maxInt - 1
	return minInt, maxInt
}

func asFloat64(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func asStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			out = append(out, item)
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				continue
			}
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			out = append(out, text)
		}
		return out
	default:
		return nil
	}
}

func coerceValue(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	switch strings.ToLower(raw) {
	case "true":
		return true
	case "false":
		return false
	}
	if integerLike(raw) {
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return value
		}
	}
	if value, err := strconv.ParseFloat(raw, 64); err == nil {
		return value
	}
	return raw
}

func integerLike(raw string) bool {
	if raw == "" {
		return false
	}
	for i, r := range raw {
		if i == 0 && (r == '-' || r == '+') {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func splitCompatibility(raw string) []string {
	parts := strings.Split(raw, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func makeRecordID(record map[string]any, rowIndex int) string {
	brand, _ := record[normalizeColumn("Brand")].(string)
	model, _ := record[normalizeColumn("Model")].(string)
	if brand == "" && model == "" {
		return fmt.Sprintf("row-%03d", rowIndex+1)
	}
	base := normalizeColumn(brand + "_" + model)
	if base == "" {
		return fmt.Sprintf("row-%03d", rowIndex+1)
	}
	return base
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
