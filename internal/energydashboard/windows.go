package energydashboard

import (
	"fmt"
	"strings"
	"time"
)

type Preset string

const (
	PresetToday        Preset = "today"
	PresetYesterday    Preset = "yesterday"
	PresetLast7Days    Preset = "last7d"
	PresetThisWeek     Preset = "thisWeek"
	PresetPreviousWeek Preset = "previousWeek"
	PresetThisMonth    Preset = "thisMonth"
	PresetLast12Months Preset = "last12m"
)

type Window struct {
	From         time.Time
	To           time.Time
	PreviousFrom time.Time
	PreviousTo   time.Time
}

func ParsePreset(raw string) (Preset, error) {
	switch Preset(strings.TrimSpace(raw)) {
	case PresetToday,
		PresetYesterday,
		PresetLast7Days,
		PresetThisWeek,
		PresetPreviousWeek,
		PresetThisMonth,
		PresetLast12Months:
		return Preset(raw), nil
	default:
		return "", fmt.Errorf("invalid energy preset: %s", raw)
	}
}

func ResolveWindow(now time.Time, loc *time.Location, preset Preset) (Window, error) {
	if loc == nil {
		loc = time.UTC
	}
	localNow := now.In(loc)
	switch preset {
	case PresetToday:
		from := startOfDay(localNow)
		return newWindow(from, localNow, from.AddDate(0, 0, -1), from), nil
	case PresetYesterday:
		to := startOfDay(localNow)
		from := to.AddDate(0, 0, -1)
		return newWindow(from, to, from.AddDate(0, 0, -1), from), nil
	case PresetLast7Days:
		from := startOfDay(localNow).AddDate(0, 0, -6)
		return newWindow(from, localNow, from.AddDate(0, 0, -7), from), nil
	case PresetThisWeek:
		from := startOfWeek(localNow)
		return newWindow(from, localNow, from.AddDate(0, 0, -7), from), nil
	case PresetPreviousWeek:
		to := startOfWeek(localNow)
		from := to.AddDate(0, 0, -7)
		return newWindow(from, to, from.AddDate(0, 0, -7), from), nil
	case PresetThisMonth:
		from := startOfMonth(localNow)
		return newWindow(from, localNow, from.AddDate(0, -1, 0), from), nil
	case PresetLast12Months:
		from := startOfMonth(localNow).AddDate(0, -11, 0)
		return newWindow(from, localNow, from.AddDate(-1, 0, 0), from), nil
	default:
		return Window{}, fmt.Errorf("invalid energy preset: %s", preset)
	}
}

func newWindow(from, to, previousFrom, previousTo time.Time) Window {
	return Window{
		From:         from.UTC(),
		To:           to.UTC(),
		PreviousFrom: previousFrom.UTC(),
		PreviousTo:   previousTo.UTC(),
	}
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// v1 uses Sunday-start weeks, matching the repo owner's locale and current
// desktop environment expectations for the Energy page presets.
func startOfWeek(t time.Time) time.Time {
	start := startOfDay(t)
	return start.AddDate(0, 0, -int(start.Weekday()))
}

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}
