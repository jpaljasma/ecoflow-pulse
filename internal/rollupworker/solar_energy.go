package rollupworker

import "time"

// Canonical PV updates can be sparse, so the shared rollup math carries the
// last PV value across short gaps. A bounded carry window avoids fabricating
// overnight solar when the provider stops emitting PV fields after sunset.
const DefaultSolarCarryForwardMaxGap = 20 * time.Minute

func IntegrateSolarWindow(
	start time.Time,
	end time.Time,
	lastPVAt time.Time,
	pvWatts float64,
	maxGap time.Duration,
	bucketWidth time.Duration,
	dayBucket bool,
	fn func(bucketStart time.Time, segmentStart time.Time, segmentEnd time.Time, wattHours float64),
) {
	if !end.After(start) || pvWatts <= 0 || fn == nil {
		return
	}
	if maxGap > 0 {
		maxEnd := lastPVAt.Add(maxGap)
		if end.After(maxEnd) {
			end = maxEnd
		}
	}
	if !end.After(start) {
		return
	}

	cursor := start
	for cursor.Before(end) {
		bucketStart := cursor.Truncate(bucketWidth)
		if dayBucket {
			bucketStart = time.Date(cursor.Year(), cursor.Month(), cursor.Day(), 0, 0, 0, 0, time.UTC)
		}
		segmentEnd := bucketStart.Add(bucketWidth)
		if segmentEnd.After(end) {
			segmentEnd = end
		}
		durationHours := segmentEnd.Sub(cursor).Hours()
		if durationHours > 0 {
			fn(bucketStart.UTC(), cursor.UTC(), segmentEnd.UTC(), pvWatts*durationHours)
		}
		cursor = segmentEnd
	}
}
