package helper

import (
	"testing"
	"time"
)

func TestCompareYYMMDD(t *testing.T) {
	type args struct {
		a time.Time
		b time.Time
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "00. equal date",
			args: args{
				a: time.Date(2000, 1, 1, 21, 0, 0, 0, time.UTC),
				b: time.Date(2000, 1, 1, 1, 0, 0, 0, time.UTC),
			},
			want: 0,
		},
		{
			name: "01. a before b",
			args: args{
				a: time.Date(2000, 1, 1, 21, 0, 0, 0, time.UTC),
				b: time.Date(2000, 2, 1, 1, 0, 0, 0, time.UTC),
			},
			want: -1,
		},
		{
			name: "02. a after b",
			args: args{
				a: time.Date(2000, 2, 3, 21, 0, 0, 0, time.UTC),
				b: time.Date(2000, 2, 1, 1, 0, 0, 0, time.UTC),
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CompareYYMMDD(tt.args.a, tt.args.b); got != tt.want {
				t.Errorf("CompareYYMMDD() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFirstLastOfMonth(t *testing.T) {
	// Use a fixed location to avoid daylight-saving edge cases in the tests
	jkt := time.FixedZone("WIB", 7*60*60) // UTC+7 (Asia/Jakarta)

	cases := []struct {
		name      string
		input     time.Time
		wantFirst time.Time
		wantLast  time.Time
	}{
		{
			name:      "31-day month (Mar 2025)",
			input:     time.Date(2025, 3, 15, 13, 37, 59, 123, time.UTC),
			wantFirst: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),
			wantLast:  time.Date(2025, 3, 31, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC),
		},
		{
			name:      "30-day month (Apr 2025)",
			input:     time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
			wantFirst: time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
			wantLast:  time.Date(2025, 4, 30, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC),
		},
		{
			name:      "Common-year Feb (2025)",
			input:     time.Date(2025, 2, 28, 22, 0, 0, 0, time.UTC),
			wantFirst: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			wantLast:  time.Date(2025, 2, 28, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC),
		},
		{
			name:      "Leap-year Feb (2024)",
			input:     time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC),
			wantFirst: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			wantLast:  time.Date(2024, 2, 29, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC),
		},
		{
			name:      "Non-UTC location (Jakarta, Aug 2025)",
			input:     time.Date(2025, 8, 20, 9, 0, 0, 0, jkt),
			wantFirst: time.Date(2025, 8, 1, 0, 0, 0, 0, jkt),
			wantLast:  time.Date(2025, 8, 31, 23, 59, 59, int(time.Second-time.Nanosecond), jkt),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotFirst, gotLast := FirstLastOfMonth(tc.input)

			if !gotFirst.Equal(tc.wantFirst) {
				t.Errorf("first day mismatch: got %v, want %v", gotFirst, tc.wantFirst)
			}
			if !gotLast.Equal(tc.wantLast) {
				t.Errorf("last day mismatch: got %v, want %v", gotLast, tc.wantLast)
			}
		})
	}
}
