package cursor

import (
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// space is one white space character of parserkit.Space.
const space = `[` + parserkit.Space + `]`

// wrapperPattern matches the worker's wrapper of a user text block,
// "<timestamp>Thursday, Oct 1, <year>, 6:00 AM (UTC-4)</timestamp>" LF
// "<user_query>Q</user_query>", with ASCII whitespace; tagPattern matches
// the tag's text: a day, a month, a day of the month, a year and a
// 12-hour time to the minute, then the UTC offset. Digits are ASCII only.
var (
	wrapperPattern = regexp.MustCompile(`(?s)^<timestamp>([^<]*)</timestamp>[ \t\n\r\f\v]*<user_query>(.*)` +
		`</user_query>[ \t\n\r\f\v]*\z`)
	tagPattern = regexp.MustCompile(`^` + space + `*[A-Za-z]+,` + space + `+([A-Za-z]{3})[A-Za-z]*\.?` + space +
		`+([0-9]{1,2}),` + space + `+([0-9]{4}),` + space + `+([0-9]{1,2}):([0-9]{2})` + space +
		`*([AaPp][Mm])` + space + `*\(UTC(?:([+-])([0-9]{1,2})(?::?([0-9]{2}))?)?\)` + space + `*\z`)
)

// months are the month names a tag's month starts with.
func months() []string {
	return []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}
}

// number returns the value of a field of ASCII digits, 0 for "".
func number(text string) int {
	value, _ := strconv.Atoi(text)
	return value
}

// tagTime returns a transcript's time tag as a UTC timestamp, or "" when
// it does not parse. The tag is local time to the minute with its UTC
// offset in parentheses.
func tagTime(tag string) string {
	match := tagPattern.FindStringSubmatch(tag)
	if match == nil {
		return ""
	}
	month := slices.Index(months(), strings.ToLower(match[1])) + 1
	year, day, minute := number(match[3]), number(match[2]), number(match[5])
	hour := number(match[4]) % 12
	if strings.ToLower(match[6]) == "pm" {
		hour += 12
	}
	if month == 0 || year < 1 || day < 1 || minute > 59 ||
		day > time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day() {
		return ""
	}
	offset := time.Duration(number(match[8]))*time.Hour + time.Duration(number(match[9]))*time.Minute
	if match[7] == "-" {
		offset = -offset
	}
	return timeutil.FormatUTC(time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.UTC).Add(-offset))
}

// unwrapQuery returns the query of a transcript user text block and its
// time: the text inside <user_query> and the tag's time, or the whole text
// and "" when the wrapper or its tag does not parse.
func unwrapQuery(text string) (string, string) {
	if match := wrapperPattern.FindStringSubmatch(text); match != nil {
		if when := tagTime(match[1]); when != "" {
			return match[2], when
		}
	}
	return text, ""
}

// githubRepository returns "<owner>/<repo>", less a .git suffix, when the
// host of a URL is github.com, else "". Cursor writes repoUrl without a
// scheme, so a value without "://" is read as a network path; a path or a
// longer host that merely holds github.com does not match, nor does text
// net/url cannot read as a URL.
func githubRepository(text string) string {
	if !strings.Contains(text, "://") {
		text = "//" + text
	}
	parsed, err := url.Parse(text)
	if err != nil || !strings.EqualFold(parsed.Hostname(), "github.com") {
		return ""
	}
	return strings.TrimSuffix(strings.Trim(parsed.EscapedPath(), "/"), ".git")
}
