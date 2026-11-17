package qwencode

import (
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/dedup"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// groupMessagePrefix starts the row of a quoted or recorded group message.
const groupMessagePrefix = "group message: "

// historySuffix is the suffix a group history message row ends in: the
// recordedAt of the next clear of its chat's key, the moment a prompt
// quoted it, or the word that the clear recorded no time or that none
// follows.
var historySuffix = regexp.MustCompile(
	`(?s)^(group message: .*) \((?:cleared, time not recorded|cleared (\S+)|waiting)\)\z`)

// asciiSpaceRun is a run of ASCII whitespace, which a channel quotes as one
// space.
var asciiSpaceRun = regexp.MustCompile(`[ \t\n\r\f\v]+`)

// suffixOf returns the parts of a group history file's "group message:"
// row, or nil for any other row: the text without its suffix, and the time
// of the clear that follows it, "" when there is none.
func suffixOf(row timeline.Row) []string {
	if row.TurnType != timeline.TurnSystem ||
		!strings.HasSuffix(row.SourceFile, "-group-history.jsonl") {
		return nil
	}
	return historySuffix.FindStringSubmatch(row.Text)
}

// MatchForm returns the turn type and text a row is deduplicated under. A
// "group message:" row matches without a history row's suffix and with
// each run of ASCII whitespace as one space, since the channel quotes a
// line break as a space; any other row matches as it is.
func MatchForm(row timeline.Row) (string, string) {
	text := row.Text
	if suffix := suffixOf(row); suffix != nil {
		text = suffix[1]
	}
	if strings.HasPrefix(text, groupMessagePrefix) {
		return row.TurnType, strings.Trim(asciiSpaceRun.ReplaceAllLiteralString(text, " "), " ")
	}
	return row.TurnType, row.Text
}

// MatchAnchor returns the time a group history row is matched at: the
// next clear of its chat's key, written when a prompt quoted it. A message
// no clear follows was never quoted, and one whose clear recorded no time
// cannot be placed, so neither matches a transcript row. Any other row is
// matched at its own time.
func MatchAnchor(row timeline.Row) (dedup.AnchorKind, int64) {
	suffix := suffixOf(row)
	if suffix == nil {
		return dedup.OwnTime, 0
	}
	if suffix[2] != "" {
		if millis, ok := timeutil.EpochMillis(suffix[2]); ok {
			return dedup.AtTime, millis
		}
	}
	return dedup.NeverMatch, 0
}
