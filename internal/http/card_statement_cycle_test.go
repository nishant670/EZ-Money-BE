package http

import (
	"testing"
	"time"
)

func TestClampDayToMonth(t *testing.T) {
	cases := []struct {
		name  string
		year  int
		month time.Month
		day   int
		want  string
	}{
		{"ordinary day", 2026, time.August, 5, "2026-08-05"},
		{"31st of a 31-day month", 2026, time.August, 31, "2026-08-31"},
		{"31st of a 30-day month", 2026, time.November, 31, "2026-11-30"},
		{"31st of a short February", 2026, time.February, 31, "2026-02-28"},
		{"29th of a leap February", 2028, time.February, 29, "2028-02-29"},
		{"29th of a non-leap February", 2026, time.February, 29, "2026-02-28"},
		{"day below range", 2026, time.August, 0, "2026-08-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := clampDayToMonth(tc.year, tc.month, tc.day).Format(apiDateLayout)
			if got != tc.want {
				t.Fatalf("clampDayToMonth = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestStatementCycle(t *testing.T) {
	cases := []struct {
		name          string
		statementDate string
		statementDay  int
		wantStart     string
		wantEnd       string
	}{
		{
			name:          "ordinary mid-month cycle",
			statementDate: "2026-08-05", statementDay: 5,
			wantStart: "2026-07-06", wantEnd: "2026-08-05",
		},
		{
			name:          "cycle ending on the 31st",
			statementDate: "2026-08-31", statementDay: 31,
			wantStart: "2026-08-01", wantEnd: "2026-08-31",
		},
		{
			// February clamps to the 28th, so March's cycle starts on the 1st.
			name:          "31st card crossing February",
			statementDate: "2026-03-31", statementDay: 31,
			wantStart: "2026-03-01", wantEnd: "2026-03-31",
		},
		{
			name:          "cycle crossing a year boundary",
			statementDate: "2026-01-10", statementDay: 10,
			wantStart: "2025-12-11", wantEnd: "2026-01-10",
		},
		{
			// statement_day unset: infer from the date the user typed.
			name:          "statement day inferred when unset",
			statementDate: "2026-08-05", statementDay: 0,
			wantStart: "2026-07-06", wantEnd: "2026-08-05",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := statementCycle(mustDate(t, tc.statementDate), tc.statementDay)
			if got := start.Format(apiDateLayout); got != tc.wantStart {
				t.Errorf("cycle start = %s, want %s", got, tc.wantStart)
			}
			if got := end.Format(apiDateLayout); got != tc.wantEnd {
				t.Errorf("cycle end = %s, want %s", got, tc.wantEnd)
			}
		})
	}
}

// Consecutive cycles must tile the calendar: no day belongs to two statements
// and no day belongs to none. This is what stops a spend from being counted
// twice or dropped between cycles.
func TestStatementCyclesTileWithoutGapOrOverlap(t *testing.T) {
	for _, statementDay := range []int{1, 5, 15, 28, 30, 31} {
		statementDate := clampDayToMonth(2026, time.January, statementDay)
		_, previousEnd := statementCycle(statementDate, statementDay)

		for month := 0; month < 24; month++ {
			statementDate = nextStatementDate(statementDate, statementDay)
			start, end := statementCycle(statementDate, statementDay)

			if want := previousEnd.AddDate(0, 0, 1); !start.Equal(want) {
				t.Fatalf("statement_day %d: cycle starting %s should start %s (previous ended %s)",
					statementDay, start.Format(apiDateLayout), want.Format(apiDateLayout),
					previousEnd.Format(apiDateLayout))
			}
			if !end.After(start) {
				t.Fatalf("statement_day %d: cycle %s..%s is empty or inverted",
					statementDay, start.Format(apiDateLayout), end.Format(apiDateLayout))
			}
			previousEnd = end
		}
	}
}

func TestDueDateFor(t *testing.T) {
	cases := []struct {
		name          string
		statementDate string
		dueDay        int
		want          string
	}{
		{
			name:          "due later in the same month",
			statementDate: "2026-08-05", dueDay: 25, want: "2026-08-25",
		},
		{
			name:          "due day already passed rolls to next month",
			statementDate: "2026-08-25", dueDay: 14, want: "2026-09-14",
		},
		{
			// Same day is not "after" — a bill is never due the day it bills.
			name:          "due day equal to statement day rolls forward",
			statementDate: "2026-08-05", dueDay: 5, want: "2026-09-05",
		},
		{
			name:          "due day clamped in a short month",
			statementDate: "2026-01-31", dueDay: 31, want: "2026-02-28",
		},
		{
			name:          "crossing a year boundary",
			statementDate: "2026-12-20", dueDay: 8, want: "2027-01-08",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dueDateFor(mustDate(t, tc.statementDate), tc.dueDay).Format(apiDateLayout)
			if got != tc.want {
				t.Fatalf("dueDateFor = %s, want %s", got, tc.want)
			}
		})
	}
}

// The due date must always land after the statement it settles, whatever the
// two anchor days are.
func TestDueDateAlwaysFollowsStatement(t *testing.T) {
	for statementDay := 1; statementDay <= 31; statementDay++ {
		for dueDay := 1; dueDay <= 31; dueDay++ {
			statementDate := clampDayToMonth(2026, time.January, statementDay)
			due := dueDateFor(statementDate, dueDay)
			if !due.After(statementDate) {
				t.Fatalf("statement_day %d / due_day %d: due %s does not follow statement %s",
					statementDay, dueDay, due.Format(apiDateLayout), statementDate.Format(apiDateLayout))
			}
		}
	}
}
