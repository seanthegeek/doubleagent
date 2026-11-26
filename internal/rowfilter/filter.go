package rowfilter

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// RowFilter holds the time window and patterns of the timeline command
// and counts the rows each test dropped, for the summary's filtered: line.
type RowFilter struct {
	// Since and Until are bounds from ParseBound, "" for none.
	Since, Until string
	// KeepUndated keeps rows without a timestamp when a bound is set.
	KeepUndated bool
	// Patterns are the --match expressions; a row is kept when any
	// matches somewhere in its text as it reads in full, with every shared
	// text it names in place (timeline.Row.Matches), whichever row the
	// output then writes that text in.
	Patterns []*regexp.Regexp
	// DroppedWindow, DroppedUndated and DroppedMatch count the rows
	// dropped as outside the window, as undated, and as matching no
	// pattern.
	DroppedWindow, DroppedUndated, DroppedMatch int
}

// CompilePatterns compiles the --match expressions, each prefixed with
// the (?i) flag when ignoreCase is set. The expressions are Go regular
// expressions, which run in linear time and have no backreferences or
// lookaround.
func CompilePatterns(expressions []string, ignoreCase bool) ([]*regexp.Regexp, error) {
	patterns := make([]*regexp.Regexp, 0, len(expressions))
	for _, expression := range expressions {
		source := expression
		if ignoreCase {
			source = "(?i)" + expression
		}
		pattern, err := regexp.Compile(source)
		if err != nil {
			return nil, fmt.Errorf("--match %s: %w", strconv.Quote(expression), err)
		}
		patterns = append(patterns, pattern)
	}
	return patterns, nil
}

// Active reports whether a window or a pattern is set.
func (f *RowFilter) Active() bool {
	return f.Since != "" || f.Until != "" || len(f.Patterns) > 0
}

// InWindow reports whether a row is inside the window, counting it when
// it is not. Without a bound every row is inside; with one, an undated row
// is inside only with KeepUndated.
func (f *RowFilter) InWindow(row timeline.Row) bool {
	if f.Since == "" && f.Until == "" {
		return true
	}
	timestamp := row.TimestampUTC
	if timestamp == "" {
		if !f.KeepUndated {
			f.DroppedUndated++
		}
		return f.KeepUndated
	}
	if (f.Since != "" && timestamp < f.Since) || (f.Until != "" && timestamp > f.Until) {
		f.DroppedWindow++
		return false
	}
	return true
}

// partnerKey is what a tool call and its result share: the session, the
// source file for a row without a session, and the tool_use_id.
type partnerKey struct {
	session, sourceFile, toolUseID string
}

// keyOf returns a row's partner key.
func keyOf(row timeline.Row) partnerKey {
	key := partnerKey{session: row.SessionID, toolUseID: row.ToolUseID}
	if row.SessionID == "" {
		key.sourceFile = row.SourceFile
	}
	return key
}

// Apply returns the rows inside the window that match a pattern, in order.
// A matched row with a tool_use_id keeps every row inside the window with
// the same partner key: its call or result and the rows beside them.
func (f *RowFilter) Apply(rows []timeline.Row) []timeline.Row {
	var kept []timeline.Row
	for _, row := range rows {
		if f.InWindow(row) {
			kept = append(kept, row)
		}
	}
	if len(f.Patterns) == 0 {
		return kept
	}
	hit := make([]bool, len(kept))
	partners := map[partnerKey]bool{}
	for i := range kept {
		row := &kept[i]
		for _, pattern := range f.Patterns {
			if row.Matches(pattern) {
				hit[i] = true
				break
			}
		}
		if hit[i] && row.ToolUseID != "" {
			partners[keyOf(*row)] = true
		}
	}
	var out []timeline.Row
	for i, row := range kept {
		if hit[i] || (row.ToolUseID != "" && partners[keyOf(row)]) {
			out = append(out, row)
		} else {
			f.DroppedMatch++
		}
	}
	return out
}
