package sandbox

import (
	"testing"
	"time"
)

func TestUnixTime(t *testing.T) {
	want := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)

	cases := []int64{want.Unix(), want.UnixMilli(), want.UnixMicro(), want.UnixNano()}
	for _, v := range cases {
		got := UnixTime(v)
		if !got.Equal(want) {
			t.Errorf("UnixTime(%d) = %v, want %v", v, got, want)
		}
	}

	if !UnixTime(0).IsZero() {
		t.Errorf("UnixTime(0) is not the zero time")
	}
}
