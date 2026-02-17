package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

type solarPanelIndex struct {
	SourceCSV      string                      `json:"source_csv"`
	GeneratedAtUTC string                      `json:"generated_at_utc"`
	RowCount       int                         `json:"row_count"`
	PanelCount     int                         `json:"panel_count"`
	ByPanelKey     map[string]solarPanelRecord `json:"by_panel_key"`
	ByDeviceTag    map[string][]string         `json:"by_device_tag"`
	DeviceLabels   map[string]string           `json:"device_labels"`
}

type solarPanelRecord struct {
	ID                string                             `json:"id"`
	Brand             string                             `json:"brand"`
	Model             string                             `json:"model"`
	Type              string                             `json:"type"`
	PmaxSTCW          float64                            `json:"pmax_stc_w"`
	VocV              float64                            `json:"voc_v"`
	VmpV              float64                            `json:"vmp_v"`
	ImpA              float64                            `json:"imp_a"`
	IscA              float64                            `json:"isc_a"`
	CompatibilityTags []string                           `json:"compatibility_tags"`
	Compatibility     map[string]solarPanelCompatSummary `json:"compatibility"`
}

type solarPanelCompatSummary struct {
	Label             string `json:"label"`
	Status            string `json:"status"`
	MinSeries         int    `json:"min_series"`
	MaxSeries         int    `json:"max_series"`
	CurrentClipLikely bool   `json:"current_clip_likely"`
}

func loadSolarPanelIndex(path string) (*solarPanelIndex, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read panel index: %w", err)
	}
	var db solarPanelIndex
	if err := json.Unmarshal(content, &db); err != nil {
		return nil, fmt.Errorf("decode panel index json: %w", err)
	}
	if db.ByPanelKey == nil {
		db.ByPanelKey = map[string]solarPanelRecord{}
	}
	if db.ByDeviceTag == nil {
		db.ByDeviceTag = map[string][]string{}
	}
	if db.DeviceLabels == nil {
		db.DeviceLabels = map[string]string{}
	}
	if db.PanelCount <= 0 {
		db.PanelCount = len(db.ByPanelKey)
	}
	return &db, nil
}

func inferPanelDeviceTags(device ecoflow.GeneralInfoDevice) []string {
	corpus := strings.ToLower(strings.TrimSpace(device.ProductName + " " + device.DeviceName))
	switch {
	case strings.Contains(corpus, "delta pro ultra"):
		return []string{"dpu_low", "dpu_high", "dpu_x_high"}
	case strings.Contains(corpus, "delta 2 max"):
		return []string{"d2_d2_max"}
	case strings.Contains(corpus, "delta pro 3"):
		return []string{"dp3_lv", "dp3_hv"}
	case strings.Contains(corpus, "delta 3 max"):
		return []string{"d3_max"}
	case strings.Contains(corpus, "delta pro"):
		return []string{"delta_pro"}
	default:
		return nil
	}
}

func (db *solarPanelIndex) candidatePanelsForDeviceTags(tags []string) []string {
	if db == nil || len(tags) == 0 || len(db.ByDeviceTag) == 0 {
		return nil
	}
	unique := make(map[string]struct{})
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag == "" {
			continue
		}
		for _, panelKey := range db.ByDeviceTag[tag] {
			panelKey = strings.TrimSpace(panelKey)
			if panelKey == "" {
				continue
			}
			unique[panelKey] = struct{}{}
		}
	}
	out := make([]string, 0, len(unique))
	for key := range unique {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func panelTagLabels(db *solarPanelIndex, tags []string) string {
	if len(tags) == 0 {
		return "n/a"
	}
	if db == nil || len(db.DeviceLabels) == 0 {
		return strings.Join(tags, ",")
	}
	labels := make([]string, 0, len(tags))
	for _, tag := range tags {
		if label := strings.TrimSpace(db.DeviceLabels[tag]); label != "" {
			labels = append(labels, label)
			continue
		}
		labels = append(labels, tag)
	}
	return strings.Join(labels, " | ")
}
