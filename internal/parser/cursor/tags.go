package cursor

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// space is a Python \s in a str pattern.
const space = `[` + parser.PythonSpace + `]`

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

// schemeCharacters are the characters a URL scheme is made of.
const schemeCharacters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+-."

// githubRepository returns "<owner>/<repo>", less a .git suffix, when the
// host of url is github.com, else "". Cursor writes repoUrl without a
// scheme, so a value without "://" is read as a network path; a path or a
// longer host that merely holds github.com does not match.
func githubRepository(url string) string {
	if !strings.Contains(url, "://") {
		url = "//" + url
	}
	url = strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(strings.TrimLeft(url, controlOrSpace))
	if colon := strings.IndexByte(url, ':'); colon > 0 && isASCIILetter(url[0]) &&
		strings.Trim(url[:colon], schemeCharacters) == "" {
		url = url[colon+1:]
	}
	if !strings.HasPrefix(url, "//") {
		return ""
	}
	end := strings.IndexAny(url[2:], "/?#")
	if end < 0 {
		end = len(url) - 2
	}
	netloc, rest := url[2:2+end], url[2+end:]
	host := netloc[strings.LastIndex(netloc, "@")+1:]
	if !strings.HasPrefix(host, "[") {
		host, _, _ = strings.Cut(host, ":")
	}
	if strings.ToLower(host) != "github.com" {
		return ""
	}
	rest, _, _ = strings.Cut(rest, "#")
	rest, _, _ = strings.Cut(rest, "?")
	return strings.TrimSuffix(strings.Trim(rest, "/"), ".git")
}

// controlOrSpace is what a URL loses at its start: the C0 controls and
// space.
const controlOrSpace = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\v\f\r\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17" +
	"\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f "

// isASCIILetter reports whether char is an ASCII letter.
func isASCIILetter(char byte) bool {
	return ('a' <= char && char <= 'z') || ('A' <= char && char <= 'Z')
}
