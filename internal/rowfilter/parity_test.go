//go:build parity

package rowfilter

import (
	"bytes"
	"encoding/hex"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// oracleScript reads lines of a hex-encoded value and the clock in epoch
// microseconds, and prints the analyzer's --since and --until bounds for
// each, E for a BoundError and X for any other failure. The analyzer's
// directory is the first argument, since -I ignores PYTHONPATH.
const oracleScript = `import sys
sys.path.insert(0, sys.argv[1])
from datetime import UTC, datetime, timedelta
from doubleagent.filters import BoundError, parse_bound
for line in sys.stdin:
    payload, micros = line.rstrip("\n").split(" ")
    text = bytes.fromhex(payload).decode("utf-8")
    now = datetime(1970, 1, 1, tzinfo=UTC) + timedelta(microseconds=int(micros))
    out = []
    for end in (False, True):
        try:
            out.append(parse_bound(text, end=end, now=now))
        except BoundError:
            out.append("E")
        except Exception:
            out.append("X")
    print(" ".join(out))
`

// query is one value with the clock it is read at.
type query struct {
	text string
	now  time.Time
}

// askPython runs oracleScript under PARITY_PYTHON, skipping the test when
// it is unset, and returns the two bounds Python gives for each query.
func askPython(t *testing.T, queries []query) [][2]string {
	t.Helper()
	python := os.Getenv("PARITY_PYTHON")
	if python == "" {
		t.Skip("PARITY_PYTHON is not set; it names the Python 3 interpreter the oracle runs on")
	}
	analyzer, err := filepath.Abs(filepath.Join("..", "..", "analyzer"))
	if err != nil {
		t.Fatal(err)
	}
	var input bytes.Buffer
	for _, item := range queries {
		input.WriteString(hex.EncodeToString([]byte(item.text)) + " " +
			strconv.FormatInt(item.now.UnixMicro(), 10) + "\n")
	}
	command := exec.Command(python, "-I", "-c", oracleScript, analyzer)
	command.Stdin = &input
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("oracle: %v\n%s", err, stderr.String())
	}
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if len(lines) != len(queries) {
		t.Fatalf("oracle gave %d answers for %d queries", len(lines), len(queries))
	}
	answers := make([][2]string, len(lines))
	for i, line := range lines {
		since, until, _ := strings.Cut(line, " ")
		answers[i] = [2]string{since, until}
	}
	return answers
}

// compare checks every query: a value read here must give Python's bounds,
// and with exact set a refused value must be one Python refuses too.
func compare(t *testing.T, queries []query, exact bool) {
	t.Helper()
	refused := 0
	for i, answer := range askPython(t, queries) {
		since, sinceErr := ParseBound(queries[i].text, false, queries[i].now)
		until, untilErr := ParseBound(queries[i].text, true, queries[i].now)
		if answer[1] == "X" && answer[0] == since {
			// Python failed computing the end of a period that ends with
			// the year 9999; the bound here is checked by its own test.
			continue
		}
		switch {
		case sinceErr == nil && untilErr == nil && [2]string{since, until} == answer:
		case sinceErr != nil && untilErr != nil && answer == [2]string{"E", "E"}:
		case sinceErr != nil && untilErr != nil && !exact:
			refused++
			if testing.Verbose() {
				t.Logf("refused %q, which Python reads as %q", queries[i].text, answer)
			}
		default:
			t.Errorf("ParseBound(%q) at %s = %q %v, %q %v; Python gives %q", queries[i].text,
				queries[i].now.Format(time.RFC3339Nano), since, sinceErr, until, untilErr, answer)
		}
	}
	t.Logf("%d of %d values Python reads are refused here", refused, len(queries))
}

// randomNow returns a clock time between the years 1900 and 2100, some at
// the end of a month.
func randomNow(random *rand.Rand) time.Time {
	moment := time.Unix(random.Int64N(6_311_390_400)-2_208_988_800,
		random.Int64N(1_000_000)*1000).UTC()
	if random.IntN(3) == 0 {
		moment = time.Date(moment.Year(), moment.Month()+1, 0, moment.Hour(), moment.Minute(),
			moment.Second(), moment.Nanosecond(), time.UTC)
	}
	return moment
}

// relativeForm returns a random relative time, day word or epoch number.
func relativeForm(random *rand.Rand) string {
	pick := func(options ...string) string { return options[random.IntN(len(options))] }
	count := strconv.Itoa(random.IntN(1_000_000))
	if random.IntN(2) == 0 {
		return count + pick("", " ") + pick("m", "min", "mins", "minute", "minutes", "h",
			"hour", "hours", "d", "day", "days", "w", "week", "weeks", "mo", "month", "months",
			"y", "year", "years", "M", "Mo", "YEARS") + pick("", " ago", " AGO")
	}
	digits := strconv.FormatInt(random.Int64N(1e15), 10)
	return pick("now", "today", "yesterday", "Today", "NOW", " now ", digits, digits+"."+count)
}

// dateForm returns a random date in one of the documented forms, and
// whether a time may follow it.
func dateForm(random *rand.Rand, number func(limit, width int) string) (string, bool) {
	pick := func(options ...string) string { return options[random.IntN(len(options))] }
	year, month, day := number(10000, 4), number(14, 2), number(33, 2)
	week, weekday, ordinal := number(56, 2), number(9, 1), number(370, 3)
	switch random.IntN(8) {
	case 0:
		return pick(year, year+"-"+month, year+"-W"+week, year+"W"+week), false
	}
	return pick(year+"-"+month+"-"+day, year+month+day, year+"-W"+week+"-"+weekday,
		year+"W"+week+weekday, year+"-"+ordinal, year+ordinal), true
}

// acceptedForm returns a random value in one of the accepted forms, its
// fields sometimes out of range.
func acceptedForm(random *rand.Rand) string {
	pick := func(options ...string) string { return options[random.IntN(len(options))] }
	number := func(limit, width int) string {
		text := strconv.Itoa(random.IntN(limit))
		return strings.Repeat("0", max(0, width-len(text))) + text
	}
	if random.IntN(5) < 2 {
		return relativeForm(random)
	}
	date, whole := dateForm(random, number)
	if !whole || random.IntN(4) == 0 {
		return date
	}
	colon := pick(":", "")
	text := date + pick("T", " ") + number(25, 2)
	if random.IntN(4) > 0 {
		text += colon + number(61, 2)
		if random.IntN(2) == 0 {
			text += colon + number(61, 2)
			if random.IntN(2) == 0 {
				text += "." + number(1_000_000_000, 9)[:1+random.IntN(9)]
			}
		}
	}
	offset := pick("", "Z", "z", "+"+number(25, 2), "-"+number(25, 2)+number(61, 2),
		"+"+number(25, 2)+":"+number(61, 2))
	if random.IntN(3) == 0 {
		offset = " " + pick(offset, "UTC", "utc", "Utc")
	}
	return text + offset
}

// mutate changes a few bytes of text: a replacement by a byte from a set
// that the forms hold, an insertion or a deletion.
func mutate(random *rand.Rand, text string) string {
	const alphabet = "0123456789-:.,+ TtZzWwU_\t"
	data := []byte(text)
	for range 1 + random.IntN(3) {
		position := random.IntN(len(data) + 1)
		char := alphabet[random.IntN(len(alphabet))]
		switch random.IntN(3) {
		case 0:
			if position < len(data) {
				data[position] = char
			}
		case 1:
			data = append(data[:position], append([]byte{char}, data[position:]...)...)
		default:
			if position < len(data) {
				data = append(data[:position], data[position+1:]...)
			}
		}
	}
	return string(data)
}

func TestTablesMatchPython(t *testing.T) {
	var queries []query
	for _, test := range acceptedCases() {
		queries = append(queries, query{test.text, testNow()})
	}
	for _, text := range rejectedCases() {
		queries = append(queries, query{text, testNow()})
	}
	compare(t, queries, true)
}

func TestAcceptedFormsMatchPython(t *testing.T) {
	random := rand.New(rand.NewPCG(29, 31))
	queries := make([]query, 6000)
	for i := range queries {
		queries[i] = query{acceptedForm(random), randomNow(random)}
	}
	compare(t, queries, false)
}

func TestOtherTextNeverDisagreesWithPython(t *testing.T) {
	random := rand.New(rand.NewPCG(37, 41))
	queries := make([]query, 8000)
	for i := range queries {
		queries[i] = query{mutate(random, acceptedForm(random)), randomNow(random)}
	}
	compare(t, queries, false)
}
