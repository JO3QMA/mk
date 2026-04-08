package chart

import "time"

// Clock abstracts time.Now so tests can drive the chart engine through
// deterministic timestamps without sleeping. The production binding is
// SystemClock; tests use a stub that returns a fixed instant.
type Clock interface {
	Now() time.Time
}

// SystemClock returns the real wall-clock time in UTC. The chart engine
// stores all timestamps as UTC seconds since epoch, so callers should
// not depend on the local timezone of this clock.
type SystemClock struct{}

// Now returns time.Now() converted to UTC.
func (SystemClock) Now() time.Time { return time.Now().UTC() }

// truncateToHour returns the start of the UTC hour containing t. The
// chart engine groups records by this timestamp for SpanHour buckets.
func truncateToHour(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC)
}

// truncateToDay returns the start of the UTC day containing t. The
// chart engine groups records by this timestamp for SpanDay buckets.
func truncateToDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// truncateToSpan returns the start of the bucket the timestamp belongs
// to for the given span. Used by claimCurrentLog and getChart range
// calculations.
func truncateToSpan(t time.Time, span Span) time.Time {
	if span == SpanDay {
		return truncateToDay(t)
	}
	return truncateToHour(t)
}

// stepBack subtracts `n` spans from t. SpanHour goes back by 1h * n,
// SpanDay goes back by 24h * n via day arithmetic that respects DST-like
// edge cases (we use UTC, so DST is not actually a concern).
func stepBack(t time.Time, n int, span Span) time.Time {
	if span == SpanDay {
		return t.AddDate(0, 0, -n)
	}
	return t.Add(-time.Duration(n) * time.Hour)
}
