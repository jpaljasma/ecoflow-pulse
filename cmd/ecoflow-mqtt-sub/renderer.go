package main

import (
	"strings"
	"unicode"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

const tableColspanPrefix = "__COLSPAN__:"

func renderDashboard(
	device ecoflow.GeneralInfoDevice,
	topic string,
	envelope telemetryEnvelope,
	snapshot *energySnapshot,
	minuteHistory *minuteTelemetryHistory,
	minuteCfg minuteTableConfig,
) string {
	vm := buildDashboardViewModel(device, topic, envelope, snapshot, minuteHistory, minuteCfg)
	return renderDashboardViewModel(vm)
}

func renderDashboardViewModel(vm dashboardViewModel) string {
	var builder strings.Builder
	builder.WriteString("\033[H\033[2J")
	builder.WriteString("EcoFlow Live Telemetry\n")
	builder.WriteByte('\n')
	builder.WriteString(renderASCIITable(vm.deviceHeaders, vm.deviceRows))
	builder.WriteString("\n")
	builder.WriteString(renderASCIITable(vm.packHeaders, vm.packRows))
	builder.WriteString("\n")
	for _, line := range vm.statusLines {
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	builder.WriteByte('\n')
	builder.WriteString(renderASCIITable(vm.summaryHeaders, vm.summaryRows))
	builder.WriteString("\n\n")
	builder.WriteString(renderASCIITable(vm.solarRecHeaders, vm.solarRecRows))
	builder.WriteString("\n\n")
	if len(vm.solarCandHeaders) > 0 && len(vm.solarCandRows) > 0 {
		builder.WriteString(renderASCIITable(vm.solarCandHeaders, vm.solarCandRows))
		builder.WriteString("\n\n")
	}
	builder.WriteString(renderASCIITable(vm.packDiagHeaders, vm.packDiagRows))
	builder.WriteString("\n\n")
	builder.WriteString(renderASCIITable(vm.minuteHeaders, vm.minuteRows))
	builder.WriteString("\n\n")
	builder.WriteString(renderASCIITable(vm.estimateHeaders, vm.estimateRows))
	builder.WriteString("\n")
	return builder.String()
}

func renderASCIITable(headers []string, rows [][]string) string {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = displayCellWidth(header)
	}
	for _, row := range rows {
		for i := 0; i < len(headers) && i < len(row); i++ {
			if cellWidth := displayMultilineCellWidth(row[i]); cellWidth > widths[i] {
				widths[i] = cellWidth
			}
		}
	}

	var builder strings.Builder
	builder.WriteString(renderTableBorder(widths))
	builder.WriteByte('\n')
	builder.WriteString(renderTableRow(headers, widths))
	builder.WriteByte('\n')
	builder.WriteString(renderTableBorder(widths))
	for _, row := range rows {
		rowLines := renderTableRowLines(row, widths)
		for _, line := range rowLines {
			builder.WriteByte('\n')
			builder.WriteString(line)
		}
	}
	builder.WriteByte('\n')
	builder.WriteString(renderTableBorder(widths))
	return builder.String()
}

func renderTableBorder(widths []int) string {
	var builder strings.Builder
	builder.WriteByte('+')
	for _, width := range widths {
		builder.WriteString(strings.Repeat("-", width+2))
		builder.WriteByte('+')
	}
	return builder.String()
}

func renderTableRow(cells []string, widths []int) string {
	lines := renderTableRowLines(cells, widths)
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

func renderTableRowLines(cells []string, widths []int) []string {
	maxLines := 1
	for i := 0; i < len(widths); i++ {
		if i >= len(cells) {
			continue
		}
		value := cells[i]
		if decoded, ok := decodeColspanCell(value); ok {
			if lines := len(strings.Split(decoded, "\n")); lines > maxLines {
				maxLines = lines
			}
			break
		}
		if lines := len(strings.Split(value, "\n")); lines > maxLines {
			maxLines = lines
		}
	}

	out := make([]string, 0, maxLines)
	for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
		var builder strings.Builder
		builder.WriteByte('|')
		for i := 0; i < len(widths); i++ {
			width := widths[i]
			cell := ""
			if i < len(cells) && i >= 0 {
				if value, ok := decodeColspanCell(cells[i]); ok {
					mergedWidth := width
					for j := i + 1; j < len(widths); j++ {
						mergedWidth += widths[j] + 3
					}
					colspanLines := strings.Split(value, "\n")
					if lineIdx < len(colspanLines) {
						cell = colspanLines[lineIdx]
					}
					builder.WriteByte(' ')
					builder.WriteString(cell)
					if pad := mergedWidth - displayCellWidth(cell); pad > 0 {
						builder.WriteString(strings.Repeat(" ", pad))
					}
					builder.WriteByte(' ')
					builder.WriteByte('|')
					break
				}
				cellLines := strings.Split(cells[i], "\n")
				if lineIdx < len(cellLines) {
					cell = cellLines[lineIdx]
				}
			}
			builder.WriteByte(' ')
			builder.WriteString(cell)
			if pad := width - displayCellWidth(cell); pad > 0 {
				builder.WriteString(strings.Repeat(" ", pad))
			}
			builder.WriteByte(' ')
			builder.WriteByte('|')
		}
		out = append(out, builder.String())
	}
	return out
}

func displayMultilineCellWidth(value string) int {
	maxWidth := 0
	for _, line := range strings.Split(value, "\n") {
		if width := displayCellWidth(line); width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}

func makeColspanCell(value string) string {
	return tableColspanPrefix + value
}

func decodeColspanCell(value string) (string, bool) {
	if !strings.HasPrefix(value, tableColspanPrefix) {
		return "", false
	}
	return strings.TrimPrefix(value, tableColspanPrefix), true
}

func displayCellWidth(value string) int {
	width := 0
	for _, r := range value {
		width += runeDisplayWidth(r)
	}
	return width
}

func runeDisplayWidth(r rune) int {
	switch {
	case r == 0:
		return 0
	case r < 0x20:
		return 0
	case r >= 0x7f && r < 0xa0:
		return 0
	case r == 0x200d || r == 0xfe0f:
		return 0 // zero-width joiner / variation selector
	case unicode.Is(unicode.Mn, r):
		return 0 // combining mark
	case isWideRune(r):
		return 2
	default:
		return 1
	}
}

func isWideRune(r rune) bool {
	switch {
	// East Asian wide / fullwidth ranges.
	case r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a):
		return true
	case r >= 0x2e80 && r <= 0xa4cf && r != 0x303f:
		return true
	case r >= 0xac00 && r <= 0xd7a3:
		return true
	case r >= 0xf900 && r <= 0xfaff:
		return true
	case r >= 0xfe10 && r <= 0xfe19:
		return true
	case r >= 0xfe30 && r <= 0xfe6f:
		return true
	case r >= 0xff00 && r <= 0xff60:
		return true
	case r >= 0xffe0 && r <= 0xffe6:
		return true
	case r >= 0x20000 && r <= 0x2fffd:
		return true
	case r >= 0x30000 && r <= 0x3fffd:
		return true

	// Emoji / symbols commonly rendered as wide in terminals.
	case r >= 0x1f300 && r <= 0x1faff:
		return true
	case r >= 0x2600 && r <= 0x27bf:
		return true
	default:
		return false
	}
}
