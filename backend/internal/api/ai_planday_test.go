package api

import "testing"

func TestParseHM(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"09:30", 9*60 + 30, true},
		{"14:00", 14 * 60, true},
		{"08:15", 8*60 + 15, true},
		{"00:00", 0, true},
		{" 13:05 ", 13*60 + 5, true}, // trimmed
		{"2026-06-19", 0, false},     // not HH:MM
		{"", 0, false},
		{"garbage", 0, false},
	}
	for _, c := range cases {
		got, ok := parseHM(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseHM(%q) = (%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestRoundUp5(t *testing.T) {
	for in, want := range map[int]int{540: 540, 541: 545, 544: 545, 545: 545, 546: 550} {
		if got := roundUp5(in); got != want {
			t.Errorf("roundUp5(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestFmtHM(t *testing.T) {
	for in, want := range map[int]string{540: "09:00", 9*60 + 5: "09:05", 13*60 + 30: "13:30", 24 * 60: "23:59"} {
		if got := fmtHM(in); got != want {
			t.Errorf("fmtHM(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestNextFreeSlot(t *testing.T) {
	// 10:00–11:00 meeting.
	busy := [][2]int{{10 * 60, 11 * 60}}

	// A 30-min task at 09:30 fits before the meeting — unchanged.
	if got := nextFreeSlot(9*60+30, 30, busy); got != 9*60+30 {
		t.Errorf("pre-meeting slot = %d, want %d", got, 9*60+30)
	}
	// A 60-min task starting 09:30 would overlap the 10:00 meeting → bumped to 11:00.
	if got := nextFreeSlot(9*60+30, 60, busy); got != 11*60 {
		t.Errorf("overlapping slot bumped to %d, want %d", got, 11*60)
	}
	// A task starting inside the meeting is pushed to its end.
	if got := nextFreeSlot(10*60+15, 30, busy); got != 11*60 {
		t.Errorf("inside-meeting slot = %d, want %d", got, 11*60)
	}
}

// Exercises the full packing walk: ordered tasks slotted around a midday meeting.
func TestSchedulePackingWalk(t *testing.T) {
	busy := [][2]int{{12 * 60, 13 * 60}} // lunch meeting 12:00–13:00
	durations := []int{90, 60, 45}       // three tasks in order
	cursor := 9 * 60
	var starts []string
	for _, dur := range durations {
		cursor = roundUp5(cursor)
		cursor = nextFreeSlot(cursor, dur, busy)
		starts = append(starts, fmtHM(cursor))
		cursor += dur
	}
	// 09:00 (90m→10:30), 10:30 (60m→11:30), then 45m at 11:30 would hit lunch →
	// bumped to 13:00.
	want := []string{"09:00", "10:30", "13:00"}
	for i := range want {
		if starts[i] != want[i] {
			t.Errorf("start[%d] = %q, want %q (all=%v)", i, starts[i], want[i], starts)
		}
	}
}
