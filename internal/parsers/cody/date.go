package cody

import (
	"net/mail"
	"strings"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// zoneOffsets are the offsets, in seconds, of the zone names RFC 5322
// section 4.3 lists; any other name says nothing of the zone and is read
// as UTC.
func zoneOffsets() map[string]int {
	const hour = 3600
	return map[string]int{"AST": -4 * hour, "ADT": -3 * hour, "EST": -5 * hour, "EDT": -4 * hour,
		"CST": -6 * hour, "CDT": -5 * hour, "MST": -7 * hour, "MDT": -6 * hour,
		"PST": -8 * hour, "PDT": -7 * hour}
}

// numericZone reports whether mail.ParseDate reads the zone of text as a
// numeric offset: after the leading spaces and tabs, the first "+" or "-"
// with four characters after it starts the zone (RFC 5322 section 3.3,
// zone = ("+" / "-") 4DIGIT), and such a date parses only as an offset.
func numericZone(text string) bool {
	rest := strings.TrimLeft(strings.ReplaceAll(text, "\r\n", ""), " \t")
	sign := strings.IndexAny(rest, "+-")
	return sign >= 0 && len(rest) >= sign+5
}

// mailDate reads a date in the RFC 5322 form through mail.ParseDate. A
// numeric offset is the instant's own. A zone given by name takes the
// offset zoneOffsets gives it, UTC for GMT, UT, UTC and every other name:
// time.Parse would look a name up in the zone of the host running the
// binary, and it names a numeric offset after that zone when the two
// agree, so neither may decide the time.
func mailDate(text string) (time.Time, bool) {
	moment, err := mail.ParseDate(text)
	if err != nil {
		return time.Time{}, false
	}
	if name, _ := moment.Zone(); name != "" && !numericZone(text) {
		year, month, day := moment.Date()
		hour, minute, second := moment.Clock()
		moment = time.Date(year, month, day, hour, minute, second, 0,
			time.FixedZone(name, zoneOffsets()[name]))
	}
	return moment, true
}

// chatTime returns the UTC time of a chat id, else of fallback; each is
// tried first as a mail date and then as any time the timeline reads, and
// only when it is a non-empty string. Cody writes the chat id with
// JavaScript's Date.toUTCString() (a day name, day, month, year, time and
// GMT), and a chat migrated from the JetBrains history keeps a zone-less
// ISO time (research/cody.md).
func chatTime(chatID string, fallback jsonvalue.Value) string {
	candidates := []string{chatID}
	if fallback.IsString() {
		candidates = append(candidates, fallback.StringValue())
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if moment, ok := mailDate(candidate); ok {
			return timeutil.FromTime(moment)
		}
		if timestamp := timeutil.FromText(candidate); timestamp != "" {
			return timestamp
		}
	}
	return ""
}
