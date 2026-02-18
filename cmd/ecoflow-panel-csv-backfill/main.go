package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

const defaultCSVPath = "data/solar_panels/solar_panel_specs_with_ecoflow_compat_cold_voc_and_safety_margins_v13.csv"

type deviceLimit struct {
	label string
	minV  float64
	maxV  float64
	maxA  float64
}

var (
	lowVoltageDevices = []deviceLimit{
		{label: "D3 Max 11-60V/13A", minV: 11, maxV: 60, maxA: 13},
		{label: "D2/D2 Max 11-60V/15A", minV: 11, maxV: 60, maxA: 15},
		{label: "D3 Ultra+ 11-60V/18A", minV: 11, maxV: 60, maxA: 18},
		{label: "DP3 LV 11-60V/20A", minV: 11, maxV: 60, maxA: 20},
		{label: "Delta Pro 11-150V/15A", minV: 11, maxV: 150, maxA: 15},
		{label: "DP3 HV 30-150V/15A", minV: 30, maxV: 150, maxA: 15},
		{label: "DPU Low 30-150V/15A", minV: 30, maxV: 150, maxA: 15},
	}
	highVoltageDevices = []deviceLimit{
		{label: "DPU High 80-450V/15A", minV: 80, maxV: 450, maxA: 15},
		{label: "DPU-X High 80-500V/15A", minV: 80, maxV: 500, maxA: 15},
	}
)

func main() {
	var csvPath string
	flag.StringVar(&csvPath, "csv", defaultCSVPath, "panel CSV path")
	flag.Parse()

	rows, header, err := readCSV(csvPath)
	if err != nil {
		fatalf("read csv: %v", err)
	}
	idx := mapHeaders(header)

	updated := 0
	for i := range rows {
		if backfillRow(rows[i], idx) {
			updated++
		}
	}

	if err := writeCSV(csvPath, header, rows); err != nil {
		fatalf("write csv: %v", err)
	}
	fmt.Printf("backfilled rows: %d/%d\n", updated, len(rows))
}

func backfillRow(row []string, idx map[string]int) bool {
	changed := false

	voc := readFloat(row, idx, "Voc_V")
	vmp := readFloat(row, idx, "Vmp_V")
	imp := readFloat(row, idx, "Imp_A")
	isc := readFloat(row, idx, "Isc_A")
	tvoc := readFloat(row, idx, "TempCoeff_Voc_%/C")
	tvocMissing := false
	if math.IsNaN(tvoc) {
		tvoc = fallbackVocCoeff(row, idx)
		tvocMissing = true
	}

	if setIfEmpty(row, idx, "Voc_temp_coeff_missing", boolString(tvocMissing)) {
		changed = true
	}

	voc0 := readFloat(row, idx, "Voc_0C_V")
	vocM20 := readFloat(row, idx, "Voc_-20C_V")
	vocM25 := readFloat(row, idx, "Voc_-25C_V")
	if !math.IsNaN(voc) {
		if math.IsNaN(voc0) {
			voc0 = vocAtTemp(voc, tvoc, 0)
			if setIfEmpty(row, idx, "Voc_0C_V", fmtNum(voc0)) {
				changed = true
			}
		}
		if math.IsNaN(vocM20) {
			vocM20 = vocAtTemp(voc, tvoc, -20)
			if setIfEmpty(row, idx, "Voc_-20C_V", fmtNum(vocM20)) {
				changed = true
			}
		}
		if math.IsNaN(vocM25) {
			vocM25 = vocAtTemp(voc, tvoc, -25)
			if setIfEmpty(row, idx, "Voc_-25C_V", fmtNum(vocM25)) {
				changed = true
			}
		}
	}

	safetyVoc := firstFinite(vocM25, vocM20, voc0, voc)
	if !math.IsNaN(safetyVoc) {
		if setIfEmpty(row, idx, "Voc_safety_basis", "Voc@-25C") {
			changed = true
		}
		if setIfEmpty(row, idx, "Voc_safety_V", fmtNum(safetyVoc)) {
			changed = true
		}
	}

	if !math.IsNaN(safetyVoc) {
		for _, lim := range []struct {
			lim       float64
			maxSeries string
			headroom  string
			margin    string
		}{
			{60, "MaxSeries_60V", "Headroom_60V_V", "SinglePanelMargin_60V_V"},
			{150, "MaxSeries_150V", "Headroom_150V_V", "SinglePanelMargin_150V_V"},
			{450, "MaxSeries_450V", "Headroom_450V_V", "SinglePanelMargin_450V_V"},
			{500, "MaxSeries_500V", "Headroom_500V_V", "SinglePanelMargin_500V_V"},
		} {
			ms := int(math.Floor(lim.lim / safetyVoc))
			if ms < 0 {
				ms = 0
			}
			if setIfEmpty(row, idx, lim.maxSeries, fmt.Sprintf("%d", ms)) {
				changed = true
			}
			if setIfEmpty(row, idx, lim.headroom, fmtNum(lim.lim-float64(ms)*safetyVoc)) {
				changed = true
			}
			if setIfEmpty(row, idx, lim.margin, fmtNum(lim.lim-safetyVoc)) {
				changed = true
			}
		}
	}

	if !math.IsNaN(vmp) && vmp > 0 {
		min80 := int(math.Ceil(80.0 / vmp))
		if min80 < 1 {
			min80 = 1
		}
		if setIfEmpty(row, idx, "MinSeries_for_80Vmp", fmt.Sprintf("%d", min80)) {
			changed = true
		}
		if setIfEmpty(row, idx, "MinSeries_Vmp_total_V", fmtNum(float64(min80)*vmp)) {
			changed = true
		}
		if !math.IsNaN(safetyVoc) {
			if setIfEmpty(row, idx, "MinSeries_Voc_total_V", fmtNum(float64(min80)*safetyVoc)) {
				changed = true
			}
			max450 := readInt(row, idx, "MaxSeries_450V")
			max500 := readInt(row, idx, "MaxSeries_500V")
			if max450 > 0 {
				if setIfEmpty(row, idx, "SeriesRange_DPU_High_450V", fmt.Sprintf("%dS-%dS", min80, max450)) {
					changed = true
				}
				if setIfEmpty(row, idx, "MinSeries_fits_450V", boolFloatString(min80 <= max450)) {
					changed = true
				}
			}
			if max500 > 0 {
				if setIfEmpty(row, idx, "SeriesRange_DPUX_High_500V", fmt.Sprintf("%dS-%dS", min80, max500)) {
					changed = true
				}
				if setIfEmpty(row, idx, "MinSeries_fits_500V", boolFloatString(min80 <= max500)) {
					changed = true
				}
			}
		}
	}

	if strings.TrimSpace(readString(row, idx, "EcoFlow_compatibility")) == "" {
		compat := buildCompat(voc, vmp, chooseCurrent(imp, isc), safetyVoc)
		if compat != "" {
			writeString(row, idx, "EcoFlow_compatibility", compat)
			changed = true
		}
	}

	return changed
}

func buildCompat(voc, vmp, current, safetyVoc float64) string {
	if math.IsNaN(voc) {
		return ""
	}
	entries := make([]string, 0, 10)
	for _, d := range lowVoltageDevices {
		status := "NO"
		if voc <= d.maxV {
			if voc >= d.minV {
				if !math.IsNaN(current) && current > d.maxA {
					status = "YES (current clip likely)"
				} else {
					status = "YES"
				}
			} else {
				status = "NO"
			}
		}
		entries = append(entries, fmt.Sprintf("%s: %s", d.label, status))
	}

	for _, d := range highVoltageDevices {
		minS := minSeriesFor80(vmp, voc)
		maxS := 0
		if !math.IsNaN(safetyVoc) && safetyVoc > 0 {
			maxS = int(math.Floor(d.maxV / safetyVoc))
		}
		status := "NO"
		if minS > 0 && maxS >= minS {
			status = fmt.Sprintf("needs >=%dS (max %dS)", minS, maxS)
		}
		entries = append(entries, fmt.Sprintf("%s: %s", d.label, status))
	}
	return strings.Join(entries, " | ")
}

func minSeriesFor80(vmp, voc float64) int {
	if !math.IsNaN(vmp) && vmp > 0 {
		m := int(math.Ceil(80 / vmp))
		if m < 1 {
			m = 1
		}
		return m
	}
	if !math.IsNaN(voc) && voc > 0 {
		m := int(math.Ceil(80 / voc))
		if m < 1 {
			m = 1
		}
		return m
	}
	return 0
}

func fallbackVocCoeff(row []string, idx map[string]int) float64 {
	typeText := strings.ToLower(readString(row, idx, "Type") + " " + readString(row, idx, "Notes"))
	if strings.Contains(typeText, "bifacial") || strings.Contains(typeText, "topcon") || strings.Contains(typeText, "n-type") {
		return -0.30
	}
	return -0.28
}

func vocAtTemp(voc, tempCoeffPct, tempC float64) float64 {
	// Voc(T) = Voc(STC) * (1 + coeff * (T-25C)); coeff in %/C.
	factor := 1.0 + (tempCoeffPct/100.0)*(tempC-25.0)
	return voc * factor
}

func chooseCurrent(imp, isc float64) float64 {
	if !math.IsNaN(imp) {
		return imp
	}
	if !math.IsNaN(isc) {
		return isc
	}
	return math.NaN()
}

func readCSV(path string) ([][]string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	recs, err := r.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(recs) < 2 {
		return nil, nil, fmt.Errorf("csv has no rows")
	}
	h := recs[0]
	rows := make([][]string, 0, len(recs)-1)
	for _, rec := range recs[1:] {
		row := make([]string, len(h))
		copy(row, rec)
		if len(row) < len(h) {
			row = append(row, make([]string, len(h)-len(row))...)
		}
		if len(row) > len(h) {
			row = row[:len(h)]
		}
		rows = append(rows, row)
	}
	return rows, h, nil
}

func writeCSV(path string, header []string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func mapHeaders(header []string) map[string]int {
	m := map[string]int{}
	for i, h := range header {
		m[strings.TrimSpace(h)] = i
	}
	return m
}

func readString(row []string, idx map[string]int, col string) string {
	i, ok := idx[col]
	if !ok || i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func writeString(row []string, idx map[string]int, col, v string) {
	i, ok := idx[col]
	if !ok || i < 0 || i >= len(row) {
		return
	}
	row[i] = v
}

func setIfEmpty(row []string, idx map[string]int, col, v string) bool {
	if strings.TrimSpace(readString(row, idx, col)) != "" {
		return false
	}
	writeString(row, idx, col, v)
	return true
}

func readFloat(row []string, idx map[string]int, col string) float64 {
	s := readString(row, idx, col)
	if s == "" {
		return math.NaN()
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN()
	}
	return v
}

func readInt(row []string, idx map[string]int, col string) int {
	s := readString(row, idx, col)
	if s == "" {
		return 0
	}
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		f, ferr := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if ferr != nil {
			return 0
		}
		return int(f)
	}
	return v
}

func fmtNum(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return ""
	}
	s := strconv.FormatFloat(v, 'f', 3, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "-0" {
		return "0"
	}
	return s
}

func boolString(v bool) string {
	if v {
		return "True"
	}
	return "False"
}

func boolFloatString(v bool) string {
	if v {
		return "1.0"
	}
	return "0.0"
}

func firstFinite(vals ...float64) float64 {
	for _, v := range vals {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			return v
		}
	}
	return math.NaN()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
