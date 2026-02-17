package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

var panelVoltageRangePattern = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*[–-]\s*([0-9]+(?:\.[0-9]+)?)\s*v`)
var panelAmpLimitPattern = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*a`)

const coldVocRiseFactor = 1.20

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

func applyPanelDBPortMetadata(snapshot *energySnapshot, db *solarPanelIndex, device ecoflow.GeneralInfoDevice) {
	if snapshot == nil || db == nil {
		return
	}
	assign := func(channel string) {
		candidates := topPanelCandidatesForChannel(db, device, channel, 0)
		assignPanelDBCandidatesToSnapshot(snapshot, channel, candidates)
		if len(candidates) == 0 {
			return
		}
		best, hasBest, alt, hasAlt := selectPanelCandidatesForMetadata(candidates)
		if !hasBest {
			return
		}
		label := strings.TrimSpace(strings.TrimSpace(best.record.Brand + " " + best.record.Model))
		if channel == "high" {
			if label != "" {
				snapshot.PVHighBestPanelLabel = label
				snapshot.HasPVHighBestPanelLabel = true
			}
			if best.record.PmaxSTCW > 0 {
				snapshot.PVHighBestPanelWatts = best.record.PmaxSTCW
				snapshot.HasPVHighBestPanelWatts = true
			}
			if best.record.VocV > 0 {
				snapshot.PVHighBestPanelVocV = best.record.VocV
				snapshot.HasPVHighBestPanelVocV = true
			}
			if best.record.VmpV > 0 {
				snapshot.PVHighBestPanelVmpV = best.record.VmpV
				snapshot.HasPVHighBestPanelVmpV = true
			}
			if best.record.ImpA > 0 {
				snapshot.PVHighBestPanelImpA = best.record.ImpA
				snapshot.HasPVHighBestPanelImpA = true
			}
			if best.record.IscA > 0 {
				snapshot.PVHighBestPanelIscA = best.record.IscA
				snapshot.HasPVHighBestPanelIscA = true
			}
			if best.maxSeries > 0 {
				snapshot.PVHighBestPanelMaxSeries = best.maxSeries
				snapshot.HasPVHighBestPanelSeries = true
			}
			snapshot.PVHighBestPanelBifacial = panelRecordIsBifacial(best.record)
			snapshot.HasPVHighBestPanelType = true
			if hasAlt {
				altLabel := strings.TrimSpace(strings.TrimSpace(alt.record.Brand + " " + alt.record.Model))
				if altLabel != "" {
					snapshot.PVHighAltPanelLabel = altLabel
					snapshot.HasPVHighAltPanelLabel = true
				}
				if alt.record.PmaxSTCW > 0 {
					snapshot.PVHighAltPanelWatts = alt.record.PmaxSTCW
					snapshot.HasPVHighAltPanelWatts = true
				}
				if alt.record.VocV > 0 {
					snapshot.PVHighAltPanelVocV = alt.record.VocV
					snapshot.HasPVHighAltPanelVocV = true
				}
				if alt.record.VmpV > 0 {
					snapshot.PVHighAltPanelVmpV = alt.record.VmpV
					snapshot.HasPVHighAltPanelVmpV = true
				}
				if alt.record.ImpA > 0 {
					snapshot.PVHighAltPanelImpA = alt.record.ImpA
					snapshot.HasPVHighAltPanelImpA = true
				}
				if alt.record.IscA > 0 {
					snapshot.PVHighAltPanelIscA = alt.record.IscA
					snapshot.HasPVHighAltPanelIscA = true
				}
				if alt.maxSeries > 0 {
					snapshot.PVHighAltPanelMaxSeries = alt.maxSeries
					snapshot.HasPVHighAltPanelSeries = true
				}
				snapshot.PVHighAltPanelBifacial = panelRecordIsBifacial(alt.record)
				snapshot.HasPVHighAltPanelType = true
			}
			return
		}
		if label != "" {
			snapshot.PVLowBestPanelLabel = label
			snapshot.HasPVLowBestPanelLabel = true
		}
		if best.record.PmaxSTCW > 0 {
			snapshot.PVLowBestPanelWatts = best.record.PmaxSTCW
			snapshot.HasPVLowBestPanelWatts = true
		}
		if best.record.VocV > 0 {
			snapshot.PVLowBestPanelVocV = best.record.VocV
			snapshot.HasPVLowBestPanelVocV = true
		}
		if best.record.VmpV > 0 {
			snapshot.PVLowBestPanelVmpV = best.record.VmpV
			snapshot.HasPVLowBestPanelVmpV = true
		}
		if best.record.ImpA > 0 {
			snapshot.PVLowBestPanelImpA = best.record.ImpA
			snapshot.HasPVLowBestPanelImpA = true
		}
		if best.record.IscA > 0 {
			snapshot.PVLowBestPanelIscA = best.record.IscA
			snapshot.HasPVLowBestPanelIscA = true
		}
		if best.maxSeries > 0 {
			snapshot.PVLowBestPanelMaxSeries = best.maxSeries
			snapshot.HasPVLowBestPanelSeries = true
		}
		snapshot.PVLowBestPanelBifacial = panelRecordIsBifacial(best.record)
		snapshot.HasPVLowBestPanelType = true
		if hasAlt {
			altLabel := strings.TrimSpace(strings.TrimSpace(alt.record.Brand + " " + alt.record.Model))
			if altLabel != "" {
				snapshot.PVLowAltPanelLabel = altLabel
				snapshot.HasPVLowAltPanelLabel = true
			}
			if alt.record.PmaxSTCW > 0 {
				snapshot.PVLowAltPanelWatts = alt.record.PmaxSTCW
				snapshot.HasPVLowAltPanelWatts = true
			}
			if alt.record.VocV > 0 {
				snapshot.PVLowAltPanelVocV = alt.record.VocV
				snapshot.HasPVLowAltPanelVocV = true
			}
			if alt.record.VmpV > 0 {
				snapshot.PVLowAltPanelVmpV = alt.record.VmpV
				snapshot.HasPVLowAltPanelVmpV = true
			}
			if alt.record.ImpA > 0 {
				snapshot.PVLowAltPanelImpA = alt.record.ImpA
				snapshot.HasPVLowAltPanelImpA = true
			}
			if alt.record.IscA > 0 {
				snapshot.PVLowAltPanelIscA = alt.record.IscA
				snapshot.HasPVLowAltPanelIscA = true
			}
			if alt.maxSeries > 0 {
				snapshot.PVLowAltPanelMaxSeries = alt.maxSeries
				snapshot.HasPVLowAltPanelSeries = true
			}
			snapshot.PVLowAltPanelBifacial = panelRecordIsBifacial(alt.record)
			snapshot.HasPVLowAltPanelType = true
		}
	}

	assign("low")
	assign("high")
}

func selectPanelCandidatesForMetadata(candidates []panelChannelCandidate) (best panelChannelCandidate, hasBest bool, alt panelChannelCandidate, hasAlt bool) {
	if len(candidates) == 0 {
		return panelChannelCandidate{}, false, panelChannelCandidate{}, false
	}
	best = candidates[0]
	hasBest = true
	for i := 1; i < len(candidates); i++ {
		if candidateIsDistinctForMetadata(best, candidates[i]) {
			return best, true, candidates[i], true
		}
	}
	if len(candidates) > 1 {
		return best, true, candidates[1], true
	}
	return best, true, panelChannelCandidate{}, false
}

func candidateIsDistinctForMetadata(best panelChannelCandidate, alt panelChannelCandidate) bool {
	bestName := strings.ToLower(strings.TrimSpace(best.record.Brand + " " + best.record.Model))
	altName := strings.ToLower(strings.TrimSpace(alt.record.Brand + " " + alt.record.Model))
	if bestName != "" && altName != "" && bestName == altName {
		return false
	}
	bestWatts := best.record.PmaxSTCW
	altWatts := alt.record.PmaxSTCW
	if bestWatts > 0 && altWatts > 0 {
		ratio := altWatts / bestWatts
		if ratio >= 0.95 && ratio <= 1.05 && alt.maxSeries == best.maxSeries {
			return false
		}
	}
	return true
}

type panelChannelCandidate struct {
	record    solarPanelRecord
	status    string
	minSeries int
	maxSeries int
	score     int
}

func topPanelCandidatesForChannel(db *solarPanelIndex, device ecoflow.GeneralInfoDevice, channel string, limit int) []panelChannelCandidate {
	if db == nil {
		return nil
	}
	if limit <= 0 {
		limit = math.MaxInt
	}
	tags := inferPanelTagsForChannel(device, channel)
	if len(tags) == 0 {
		return nil
	}

	candidatesByID := make(map[string]panelChannelCandidate)
	for _, tag := range tags {
		panelKeys := db.ByDeviceTag[strings.ToLower(strings.TrimSpace(tag))]
		for _, panelKey := range panelKeys {
			record, ok := db.ByPanelKey[panelKey]
			if !ok {
				continue
			}
			compat := record.Compatibility[strings.ToLower(strings.TrimSpace(tag))]
			score := compatibilityStatusScore(compat.Status)
			if score <= 0 {
				continue
			}
			watts := record.PmaxSTCW
			if watts <= 0 {
				continue
			}
			maxSeries := compat.MaxSeries
			if coldLimited, ok := coldSafeMaxSeries(compat.Label, record.VocV); ok {
				if coldLimited < 1 {
					// Skip unsafe options where a single panel can exceed cold-weather MPPT voltage.
					continue
				}
				if maxSeries <= 0 || coldLimited < maxSeries {
					maxSeries = coldLimited
				}
			}
			if maxAmps, ok := parseCompatibilityMaxAmps(compat.Label); ok && maxAmps > 0 {
				panelCurrent := panelCurrentForCompatibility(record)
				// Use a small epsilon to avoid floating point false negatives at the limit.
				if panelCurrent > maxAmps+0.05 {
					// Skip unsafe options where a single panel exceeds MPPT current limit.
					continue
				}
			}
			candidate := panelChannelCandidate{
				record:    record,
				status:    compat.Status,
				minSeries: compat.MinSeries,
				maxSeries: maxSeries,
				score:     score,
			}
			if existing, exists := candidatesByID[record.ID]; exists {
				existingWatts := existing.record.PmaxSTCW
				if candidate.score < existing.score || (candidate.score == existing.score && watts <= existingWatts) {
					continue
				}
			}
			candidatesByID[record.ID] = candidate
		}
	}
	if len(candidatesByID) == 0 {
		return nil
	}
	out := make([]panelChannelCandidate, 0, len(candidatesByID))
	for _, candidate := range candidatesByID {
		out = append(out, candidate)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		if out[i].record.PmaxSTCW != out[j].record.PmaxSTCW {
			return out[i].record.PmaxSTCW > out[j].record.PmaxSTCW
		}
		return out[i].record.ID < out[j].record.ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func assignPanelDBCandidatesToSnapshot(snapshot *energySnapshot, channel string, candidates []panelChannelCandidate) {
	if snapshot == nil {
		return
	}
	converted := make([]panelDBCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		label := strings.TrimSpace(strings.TrimSpace(candidate.record.Brand + " " + candidate.record.Model))
		if label == "" {
			continue
		}
		converted = append(converted, panelDBCandidate{
			Label:      label,
			Status:     strings.TrimSpace(candidate.status),
			PanelWatts: candidate.record.PmaxSTCW,
			VocV:       candidate.record.VocV,
			VmpV:       candidate.record.VmpV,
			ImpA:       candidate.record.ImpA,
			IscA:       candidate.record.IscA,
			MinSeries:  candidate.minSeries,
			MaxSeries:  candidate.maxSeries,
			Bifacial:   panelRecordIsBifacial(candidate.record),
		})
	}
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "high":
		snapshot.PVHighDBCandidates = converted
		snapshot.HasPVHighDBCandidates = len(converted) > 0
	default:
		snapshot.PVLowDBCandidates = converted
		snapshot.HasPVLowDBCandidates = len(converted) > 0
	}
}

func inferPanelTagsForChannel(device ecoflow.GeneralInfoDevice, channel string) []string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel != "high" {
		channel = "low"
	}
	corpus := strings.ToLower(strings.TrimSpace(device.ProductName + " " + device.DeviceName))
	switch {
	case strings.Contains(corpus, "delta pro ultra"), strings.Contains(corpus, "dpu"):
		if channel == "high" {
			return []string{"dpu_high", "dpu_x_high"}
		}
		return []string{"dpu_low"}
	case strings.Contains(corpus, "delta 2 max"):
		return []string{"d2_d2_max"}
	case strings.Contains(corpus, "delta pro 3"):
		if channel == "high" {
			return []string{"dp3_hv"}
		}
		return []string{"dp3_lv"}
	case strings.Contains(corpus, "delta 3 max"):
		return []string{"d3_max"}
	case strings.Contains(corpus, "delta pro"):
		return []string{"delta_pro"}
	default:
		return nil
	}
}

func compatibilityStatusScore(status string) int {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch {
	case normalized == "no" || strings.Contains(normalized, " no"):
		return 0
	case normalized == "yes":
		return 3
	case normalized == "needs_series":
		return 2
	case strings.Contains(normalized, "compatible"):
		return 3
	case strings.Contains(normalized, "caution"), strings.Contains(normalized, "warn"), strings.Contains(normalized, "clip"):
		return 2
	case normalized != "":
		return 1
	default:
		return 0
	}
}

func panelRecordIsBifacial(record solarPanelRecord) bool {
	corpus := strings.ToLower(strings.TrimSpace(record.Type + " " + record.Model))
	if corpus == "" {
		return false
	}
	return strings.Contains(corpus, "bifacial") || strings.Contains(corpus, "bi-facial")
}

func coldSafeMaxSeries(label string, vocV float64) (int, bool) {
	if vocV <= 0 {
		return 0, false
	}
	maxVolts, ok := parseCompatibilityMaxVolts(label)
	if !ok || maxVolts <= 0 {
		return 0, false
	}
	coldVoc := vocV * coldVocRiseFactor
	if coldVoc <= 0 {
		return 0, false
	}
	maxSeries := int(math.Floor(maxVolts / coldVoc))
	return maxSeries, true
}

func parseCompatibilityMaxVolts(label string) (float64, bool) {
	label = strings.TrimSpace(label)
	if label == "" {
		return 0, false
	}
	match := panelVoltageRangePattern.FindStringSubmatch(label)
	if len(match) != 3 {
		return 0, false
	}
	low, errLow := strconv.ParseFloat(match[1], 64)
	high, errHigh := strconv.ParseFloat(match[2], 64)
	if errLow != nil || errHigh != nil {
		return 0, false
	}
	if high < low {
		low, high = high, low
	}
	return high, true
}

func parseCompatibilityMaxAmps(label string) (float64, bool) {
	label = strings.TrimSpace(label)
	if label == "" {
		return 0, false
	}
	match := panelAmpLimitPattern.FindStringSubmatch(label)
	if len(match) != 2 {
		return 0, false
	}
	amps, err := strconv.ParseFloat(match[1], 64)
	if err != nil || amps <= 0 {
		return 0, false
	}
	return amps, true
}

func panelCurrentForCompatibility(record solarPanelRecord) float64 {
	if record.ImpA > 0 {
		return record.ImpA
	}
	if record.IscA > 0 {
		return record.IscA
	}
	return 0
}
