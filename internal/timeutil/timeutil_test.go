package timeutil

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestFromTextMatchesPythonForTheFormsAgentsWrite(t *testing.T) {
	for _, test := range textCases() {
		want, verb := test.python, "gives Python's value"
		if test.refused != "" {
			want, verb = "", "is refused as "+test.refused
		}
		t.Run(strconv.Quote(test.input)+" "+verb, func(t *testing.T) {
			if got := FromText(test.input); got != want {
				t.Errorf("FromText(%q) = %q, want %q (Python %q)", test.input, got, want,
					test.python)
			}
		})
	}
}

func TestFromEpochRoundsAsPythonDoes(t *testing.T) {
	for _, test := range epochCases() {
		epoch := math.Float64frombits(test.bits)
		t.Run(strconv.FormatFloat(epoch, 'g', -1, 64)+" gives Python's value", func(t *testing.T) {
			if got := FromEpoch(epoch); got != test.python {
				t.Errorf("FromEpoch(%v) = %q, want %q", epoch, got, test.python)
			}
		})
	}
}

func TestFromIntReadsTheNearestFloat(t *testing.T) {
	for _, test := range intCases() {
		t.Run(strconv.FormatInt(test.input, 10)+" gives Python's value", func(t *testing.T) {
			if got := FromInt(test.input); got != test.python {
				t.Errorf("FromInt(%d) = %q, want %q", test.input, got, test.python)
			}
		})
	}
}

func TestFromValueReadsEachKindAsPythonDoes(t *testing.T) {
	big, err := jsonvalue.DecodeString("12345678901234567890123456789012345678901234567890")
	if err != nil {
		t.Fatal(err)
	}
	huge, err := jsonvalue.DecodeString("1" + strings.Repeat("0", 400))
	if err != nil {
		t.Fatal(err)
	}
	infinite, err := jsonvalue.DecodeString("1e400")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		value jsonvalue.Value
		want  string
	}{
		{"null gives nothing", jsonvalue.NewNull(), ""},
		{"true gives nothing", jsonvalue.NewBool(true), ""},
		{"false gives nothing", jsonvalue.NewBool(false), ""},
		{"an ISO string is read as text", jsonvalue.NewString("2026-10-01T08:00:00+02:00"),
			"2026-10-01T06:00:00.000Z"},
		{"a digit string is an epoch", jsonvalue.NewString("1700000000"), "2023-11-14T22:13:20.000Z"},
		{"an empty string gives nothing", jsonvalue.NewString(""), ""},
		{"an integer is seconds", jsonvalue.NewInt(1700000000), "2023-11-14T22:13:20.000Z"},
		{"a large integer is milliseconds", jsonvalue.NewInt(1700000000123),
			"2023-11-14T22:13:20.123Z"},
		{"zero is the epoch", jsonvalue.NewInt(0), "1970-01-01T00:00:00.000Z"},
		{"a float is seconds", jsonvalue.NewFloat(1700000000.5), "2023-11-14T22:13:20.500Z"},
		{"an integer past the years gives nothing", big, ""},
		{"an integer past the float range gives nothing", huge, ""},
		{"a float read as infinity gives nothing", infinite, ""},
		{"NaN gives nothing", jsonvalue.NewFloat(math.NaN()), ""},
		{"an array gives nothing", jsonvalue.NewArray(jsonvalue.NewString("2026-10-01")), ""},
		{"an object gives nothing", jsonvalue.NewObject(), ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := FromValue(test.value); got != test.want {
				t.Errorf("FromValue(%s) = %q, want %q", test.value.JSON(), got, test.want)
			}
		})
	}
}

func TestEpochMillisReadsOnlyTheTimelineForm(t *testing.T) {
	for _, test := range millisCases() {
		want, wantOK, verb := test.python, test.ok, "gives Python's value"
		if test.refused != "" {
			want, wantOK, verb = 0, false, "is refused as "+test.refused
		}
		t.Run(strconv.Quote(test.input)+" "+verb, func(t *testing.T) {
			got, ok := EpochMillis(test.input)
			if got != want || ok != wantOK {
				t.Errorf("EpochMillis(%q) = %d, %v; want %d, %v", test.input, got, ok, want, wantOK)
			}
		})
	}
}

func TestEpochMillisReadsBackWhatFormatUTCWrites(t *testing.T) {
	for _, milliseconds := range []int64{0, -1, 1, 1700000000123, -62135596800000,
		253402300799999, -86400001} {
		moment := time.UnixMilli(milliseconds)
		t.Run(FormatUTC(moment)+" reads back", func(t *testing.T) {
			got, ok := EpochMillis(FormatUTC(moment))
			if !ok || got != milliseconds {
				t.Errorf("EpochMillis(%q) = %d, %v; want %d", FormatUTC(moment), got, ok,
					milliseconds)
			}
		})
	}
}

func TestFormatUTCTruncatesToMillisecondsInUTC(t *testing.T) {
	zone := time.FixedZone("plus five thirty", 5*3600+1800)
	cases := []struct {
		name   string
		moment time.Time
		want   string
	}{
		{"a fraction is truncated, not rounded",
			time.Date(2026, 10, 1, 8, 0, 0, 999_999_999, time.UTC), "2026-10-01T08:00:00.999Z"},
		{"another zone is converted to UTC", time.Date(2026, 10, 1, 8, 0, 0, 0, zone),
			"2026-10-01T02:30:00.000Z"},
		{"the year one has four digits", time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
			"0001-01-01T00:00:00.000Z"},
		{"a time before the epoch keeps its fraction",
			time.Unix(-1, 1_500_000), "1969-12-31T23:59:59.001Z"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := FormatUTC(test.moment); got != test.want {
				t.Errorf("FormatUTC(%v) = %q, want %q", test.moment, got, test.want)
			}
		})
	}
}

func TestFirstNonEmptySkipsEmptyValues(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		want   string
	}{
		{"the first non-empty value wins", []string{"", "a", "b"}, "a"},
		{"all empty gives nothing", []string{"", ""}, ""},
		{"no values gives nothing", nil, ""},
		{"a space is not empty", []string{" ", "a"}, " "},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := FirstNonEmpty(test.values...); got != test.want {
				t.Errorf("FirstNonEmpty(%q) = %q, want %q", test.values, got, test.want)
			}
		})
	}
}

func TestClockIsTheTimeAFunctionGives(t *testing.T) {
	pinned := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	clock := Clock(func() time.Time { return pinned })
	if got := FormatUTC(clock()); got != "2026-10-01T08:00:00.000Z" {
		t.Errorf("FormatUTC(clock()) = %q", got)
	}
}
