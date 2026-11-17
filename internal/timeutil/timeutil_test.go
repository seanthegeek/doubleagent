package timeutil

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// acceptedText are text inputs in the forms the package comment lists and
// the timestamps they give.
func acceptedText() [][2]string {
	return [][2]string{
		{"2026-10-01", "2026-10-01T00:00:00.000Z"},
		{"2026-10-01T08:00", "2026-10-01T08:00:00.000Z"},
		{"2026-10-01T08:00:00", "2026-10-01T08:00:00.000Z"},
		{"2026-10-01T08:00:00.5", "2026-10-01T08:00:00.500Z"},
		{"2026-10-01T08:00:00,5", "2026-10-01T08:00:00.500Z"},
		{"2026-10-01T08:00:00.123456789012", "2026-10-01T08:00:00.123Z"},
		{"2026-10-01T08:00:00.9999999", "2026-10-01T08:00:00.999Z"},
		{"2026-10-01 08:00", "2026-10-01T08:00:00.000Z"},
		{"2026-10-01t08:00:00z", "2026-10-01T08:00:00.000Z"},
		{"2026-10-01T08:00:00Z", "2026-10-01T08:00:00.000Z"},
		{"2026-10-01T08:00Z", "2026-10-01T08:00:00.000Z"},
		{"2026-10-01T08:00:00+05:30", "2026-10-01T02:30:00.000Z"},
		{"2026-10-01T08:00:00+0530", "2026-10-01T02:30:00.000Z"},
		{"2026-10-01T08:00+05", "2026-10-01T03:00:00.000Z"},
		{"2026-10-01 08:00:00.123-07:00", "2026-10-01T15:00:00.123Z"},
		{"2026-10-01T08:00:00-23:59", "2026-10-02T07:59:00.000Z"},
		{" \t\r\x0c\x0b2026-10-01T08:00:00Z\n", "2026-10-01T08:00:00.000Z"},
		{"2024-02-29T23:59:59.999Z", "2024-02-29T23:59:59.999Z"},
		{"0001-01-01T00:30:00+00:30", "0001-01-01T00:00:00.000Z"},
		{"9999-12-31T23:59:59.999999Z", "9999-12-31T23:59:59.999Z"},
		{"1700000000", "2023-11-14T22:13:20.000Z"},
		{"1700000000.9999995", "2023-11-14T22:13:20.999Z"},
		{"1700000000123", "2023-11-14T22:13:20.123Z"},
		{"100000000000", "5138-11-16T09:46:40.000Z"},
		{"100000000000.5", "1973-03-03T09:46:40.000Z"},
		{"253402300799999", "9999-12-31T23:59:59.999Z"},
		{".5", "1970-01-01T00:00:00.500Z"},
	}
}

// refusedText are text inputs that give no timestamp, each with the reason.
func refusedText() [][2]string {
	return [][2]string{
		{"", "nothing"},
		{"2026-10-01_08:00:00", "a separator other than T, t or a space"},
		{"2026-10-01\t08:00", "a tab as separator"},
		{"2026-10-01Z", "a date with a zone but no time"},
		{"2026-10-01+05:00", "a date with an offset but no time"},
		{"2026-10-01T8:00:00", "a one-digit hour"},
		{"2026-10-01T24:00", "24:00"},
		{"2026-10-01T23:59:60", "a second of 60"},
		{"2026-10-01T08:00:00+24:00", "an offset of 24 hours"},
		{"2026-10-01T08:00:00+05:60", "offset minutes of 60"},
		{"2026-10-01T08:00:00+05:30:15", "an offset with seconds"},
		{"2026-10-01T08:00:00 +05:00", "a space before the offset"},
		{"2026-10-01T08:00:00.", "a point without a fraction"},
		{"2026-02-30", "a day past the end of its month"},
		{"0000-01-01", "the year zero"},
		{"0001-01-01T00:00:00+01:00", "a time before the year one"},
		{"9999-12-31T23:59:59-01:00", "a time after the year 9999"},
		{"2026-W40-1", "a week date"},
		{"2026-274", "an ordinal date"},
		{"20261001T080000Z", "the basic form"},
		{"2026-10-01T08:00:00GMT", "a zone name"},
		{"\u0662\u0660\u0662\u0666-10-01", "non-ASCII digits"},
		{"2026-10-01T08:00:00Z\u00a0", "a non-ASCII space"},
		{"1700000000.5.5", "two points"},
		{"253402300800000", "milliseconds after the year 9999"},
		{"99999999999999999999", "an epoch past int64"},
	}
}

func TestFromTextReadsTheListedForms(t *testing.T) {
	for _, test := range acceptedText() {
		t.Run(strconv.Quote(test[0])+" is read", func(t *testing.T) {
			if got := FromText(test[0]); got != test[1] {
				t.Errorf("FromText(%q) = %q, want %q", test[0], got, test[1])
			}
		})
	}
	for _, test := range refusedText() {
		t.Run(strconv.Quote(test[0])+" is refused: "+test[1], func(t *testing.T) {
			if got := FromText(test[0]); got != "" {
				t.Errorf("FromText(%q) = %q, want nothing", test[0], got)
			}
		})
	}
}

func TestFromEpochCutsTheShortestDecimalToTheMillisecond(t *testing.T) {
	tests := []struct {
		epoch float64
		want  string
	}{
		{0, "1970-01-01T00:00:00.000Z"},
		{math.Copysign(0, -1), "1970-01-01T00:00:00.000Z"},
		{-1, "1969-12-31T23:59:59.000Z"},
		{-0.5, "1969-12-31T23:59:59.500Z"},
		{-5e-7, "1969-12-31T23:59:59.999Z"},
		{1700000000.123, "2023-11-14T22:13:20.123Z"},
		{1700000000.9999995, "2023-11-14T22:13:20.999Z"},
		{1700000000123.9, "2023-11-14T22:13:20.123Z"},
		{-62135596800, "0001-01-01T00:00:00.000Z"},
		{-62135596800.001, ""},
		{1e21, ""},
		{math.NaN(), ""},
		{math.Inf(1), ""},
	}
	for _, test := range tests {
		t.Run(strconv.FormatFloat(test.epoch, 'g', -1, 64), func(t *testing.T) {
			if got := FromEpoch(test.epoch); got != test.want {
				t.Errorf("FromEpoch(%v) = %q, want %q", test.epoch, got, test.want)
			}
		})
	}
}

func TestFromIntReadsMillisecondsExactly(t *testing.T) {
	tests := []struct {
		epoch int64
		want  string
	}{
		{0, "1970-01-01T00:00:00.000Z"},
		{-1, "1969-12-31T23:59:59.000Z"},
		{1700000000, "2023-11-14T22:13:20.000Z"},
		{100000000000, "5138-11-16T09:46:40.000Z"},
		{100000000001, "1973-03-03T09:46:40.001Z"},
		{253402300799999, "9999-12-31T23:59:59.999Z"},
		{253402300800000, ""},
		{-62135596801, ""},
		{math.MaxInt64, ""},
		{math.MinInt64, ""},
	}
	for _, test := range tests {
		t.Run(strconv.FormatInt(test.epoch, 10), func(t *testing.T) {
			if got := FromInt(test.epoch); got != test.want {
				t.Errorf("FromInt(%d) = %q, want %q", test.epoch, got, test.want)
			}
		})
	}
}

func TestFromValueReadsEachKind(t *testing.T) {
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
		{"a number's own digits are read", decoded(t, "1700000000.1239"), "2023-11-14T22:13:20.123Z"},
		{"an exponent is read through its float", decoded(t, "1.7e9"), "2023-11-14T22:13:20.000Z"},
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
	tests := []struct {
		input string
		want  int64
		ok    bool
	}{
		{"2026-10-01T08:00:00.123Z", 1790841600123, true},
		{"1969-12-31T23:59:59.999Z", -1, true},
		{"0001-01-01T00:00:00.000Z", -62135596800000, true},
		{"9999-12-31T23:59:59.999Z", 253402300799999, true},
		{"", 0, false},
		{"2023-02-29T12:00:00.000Z", 0, false},
		{"0000-01-01T00:00:00.000Z", 0, false},
		{"2026-10-01T08:00:00Z", 0, false},
		{"2026-10-01 08:00:00.000Z", 0, false},
		{"2026-10-01T08:00:00.5Z", 0, false},
		{"2026-10-01t08:00:00.000z", 0, false},
		{"2026-10-01T08:00:00.000Z ", 0, false},
	}
	for _, test := range tests {
		t.Run(strconv.Quote(test.input), func(t *testing.T) {
			got, ok := EpochMillis(test.input)
			if got != test.want || ok != test.ok {
				t.Errorf("EpochMillis(%q) = %d, %v; want %d, %v", test.input, got, ok, test.want, test.ok)
			}
		})
	}
}

// decoded returns a JSON value, failing the test when text does not decode.
func decoded(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
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
