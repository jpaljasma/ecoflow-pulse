package energydashboard

import (
	"fmt"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
)

type CalendarDay struct {
	Date              string
	Year              int
	Month             int
	Day               int
	InSelectedMonth   bool
	HasData           bool
	IsFuture          bool
	SolarGeneratedKWh float64
	EstimatedValue    float64
	Currency          string
}

type CalendarTotals struct {
	SolarGeneratedKWh float64
	EstimatedValue    float64
	Currency          string
}

type CalendarMonth struct {
	Year                int
	Month               int
	VisibleDays         []CalendarDay
	SelectedMonthTotals CalendarTotals
}

func ResolveSelectedDateWindow(now time.Time, loc *time.Location, year, month, day int) (Window, error) {
	if loc == nil {
		loc = time.UTC
	}
	selected := time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc)
	if selected.Year() != year || int(selected.Month()) != month || selected.Day() != day {
		return Window{}, fmt.Errorf("invalid selected date: %04d-%02d-%02d", year, month, day)
	}
	localNow := now.In(loc)
	from := startOfDay(selected)
	to := from.AddDate(0, 0, 1)
	previousFrom := from.AddDate(0, 0, -1)
	previousTo := from
	if sameLocalDay(selected, localNow) {
		to = localNow
		previousTo = localNow.AddDate(0, 0, -1)
	}
	return newWindow(from, to, previousFrom, previousTo), nil
}

func BuildCalendarMonth(now time.Time, loc *time.Location, year, month int, gridPricePerKWh float64, currency string, queryRange func(from, to time.Time) (telemetryquery.Series, error)) (CalendarMonth, error) {
	if queryRange == nil {
		return CalendarMonth{}, fmt.Errorf("calendar query function is required")
	}
	if loc == nil {
		loc = time.UTC
	}

	visibleFrom, visibleTo, err := calendarMonthVisibleWindow(year, month, loc)
	if err != nil {
		return CalendarMonth{}, err
	}

	nowLocal := now.In(loc)
	todayStart := startOfDay(nowLocal)
	out := CalendarMonth{
		Year:        year,
		Month:       month,
		VisibleDays: make([]CalendarDay, 0, int(visibleTo.Sub(visibleFrom).Hours()/24)+1),
	}

	rangeSeries := telemetryquery.Series{
		From: visibleFrom.UTC(),
		To:   visibleFrom.UTC(),
	}
	dataTo := visibleTo
	if nowLocal.Before(dataTo) {
		dataTo = nowLocal
	}
	if dataTo.After(visibleFrom) {
		series, err := queryRange(visibleFrom.UTC(), dataTo.UTC())
		if err != nil {
			return CalendarMonth{}, err
		}
		rangeSeries = series
	}

	for day := visibleFrom; day.Before(visibleTo); day = day.AddDate(0, 0, 1) {
		dayStart := startOfDay(day)
		dayCell := CalendarDay{
			Date:            dayStart.Format("2006-01-02"),
			Year:            dayStart.Year(),
			Month:           int(dayStart.Month()),
			Day:             dayStart.Day(),
			InSelectedMonth: int(dayStart.Month()) == month,
			IsFuture:        dayStart.After(todayStart),
			Currency:        currency,
		}
		if dayCell.IsFuture {
			out.VisibleDays = append(out.VisibleDays, dayCell)
			continue
		}

		window, err := ResolveSelectedDateWindow(now, loc, dayStart.Year(), int(dayStart.Month()), dayStart.Day())
		if err != nil {
			return CalendarMonth{}, err
		}
		series := sliceCalendarSeries(rangeSeries, window.From.UTC(), window.To.UTC())
		totals := TotalsFromSeries(series)
		dayCell.HasData = len(series.Points) > 0
		dayCell.SolarGeneratedKWh = totals.SolarGeneratedWh / 1000
		dayCell.EstimatedValue = EstimatedGeneratedValue(totals.SolarGeneratedWh, gridPricePerKWh)
		out.VisibleDays = append(out.VisibleDays, dayCell)
		if dayCell.InSelectedMonth {
			out.SelectedMonthTotals.SolarGeneratedKWh += dayCell.SolarGeneratedKWh
			out.SelectedMonthTotals.EstimatedValue += dayCell.EstimatedValue
		}
	}

	out.SelectedMonthTotals.Currency = currency
	return out, nil
}

func sliceCalendarSeries(series telemetryquery.Series, from, to time.Time) telemetryquery.Series {
	out := telemetryquery.Series{
		DeviceID:             series.DeviceID,
		Resolution:           series.Resolution,
		From:                 from,
		To:                   to,
		EnergyBucketCoverage: series.EnergyBucketCoverage,
	}
	if !to.After(from) || len(series.Points) == 0 {
		return out
	}
	out.Points = make([]telemetryquery.Point, 0, len(series.Points))
	for _, point := range series.Points {
		if point.BucketStart.Before(from) || !point.BucketStart.Before(to) {
			continue
		}
		out.Points = append(out.Points, point)
	}
	return out
}

func calendarMonthVisibleWindow(year, month int, loc *time.Location) (time.Time, time.Time, error) {
	if month < 1 || month > 12 {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid calendar month: %d", month)
	}
	if loc == nil {
		loc = time.UTC
	}
	monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	if monthStart.Year() != year || int(monthStart.Month()) != month {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid calendar month: %d", month)
	}
	nextMonthStart := monthStart.AddDate(0, 1, 0)
	lastDay := nextMonthStart.AddDate(0, 0, -1)
	visibleFrom := startOfWeek(monthStart)
	visibleTo := startOfWeek(lastDay).AddDate(0, 0, 7)
	return visibleFrom, visibleTo, nil
}

func sameLocalDay(left, right time.Time) bool {
	return left.Year() == right.Year() && left.Month() == right.Month() && left.Day() == right.Day()
}
