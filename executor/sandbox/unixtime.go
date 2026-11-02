package sandbox

import "time"

// UnixTime reads a runtime timestamp. msb's job API does not document its
// unit, so the magnitude decides: seconds, milliseconds, microseconds or
// nanoseconds since the epoch, all of which are far apart for any date
// after 1971. Zero stays the zero time.
func UnixTime(v int64) time.Time {
	switch {
	case v <= 0:
		return time.Time{}
	case v < 1e11:
		return time.Unix(v, 0).UTC()
	case v < 1e14:
		return time.UnixMilli(v).UTC()
	case v < 1e17:
		return time.UnixMicro(v).UTC()
	default:
		return time.Unix(0, v).UTC()
	}
}
