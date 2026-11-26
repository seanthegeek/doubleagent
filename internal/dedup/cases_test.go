package dedup

import (
	"fmt"
	"slices"

	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// nearestLineCases are the tests that a line takes the primary row
// nearest to it, and a primary row the nearest line.
func nearestLineCases() []deduplicateCase {
	var cut, history []timeline.Row
	for _, second := range []int{10, 20, 30} {
		cut = append(cut, primaryRow(fmt.Sprintf("2026-10-01T10:00:%02d.500Z", second),
			set("text", "continue"), set("line", second)))
	}
	for _, second := range []int{50, 10, 20, 30, 40} {
		history = append(history, historyRow(fmt.Sprintf("2026-10-01T10:00:%02d.000Z", second),
			set("text", "continue"), set("line", second)))
	}
	first := historyRow("2026-10-01T10:00:01.500Z", set("line", 1))
	second := historyRow("2026-10-01T10:00:03.500Z", set("line", 2))
	return []deduplicateCase{
		{name: "each primary row takes the history line nearest to it",
			primary: cut, secondary: withTolerance(toleranceSeconds, history...),
			want: rows(history[0], history[4]), dropped: map[string]int{"claude-code": 3}},
		{name: "a line takes the nearest primary row, leaving the next line unmatched",
			primary:   rows(primaryRow(baseTime), primaryRow("2026-10-01T10:00:01.800Z")),
			secondary: withTolerance(toleranceSeconds, second, first), want: rows(second)},
	}
}

// windowEdgeCases are the tests of a line at and just past the edge of
// the default time window.
func windowEdgeCases() []deduplicateCase {
	return []deduplicateCase{
		{name: "a line two seconds after its primary row matches",
			primary:   rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow("2026-10-01T10:00:02.000Z"))},
		{name: "a line two seconds before its primary row matches",
			primary:   rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow("2026-10-01T09:59:58.000Z"))},
		{name: "a line one millisecond past the window after is kept",
			primary:   rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow("2026-10-01T10:00:02.001Z")),
			want:      rows(historyRow("2026-10-01T10:00:02.001Z"))},
		{name: "a line one millisecond past the window before is kept",
			primary:   rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow("2026-10-01T09:59:57.999Z")),
			want:      rows(historyRow("2026-10-01T09:59:57.999Z"))},
	}
}

// undatedLineCases are the tests of lines and primary rows with no
// timestamp, and of the pairing of an undated line one to one.
func undatedLineCases() []deduplicateCase {
	late := primaryRow("2026-10-01T11:00:00.000Z", set("line", 9))
	early := primaryRow("2026-10-01T09:00:00.000Z", set("line", 4))
	dated := historyRow("2026-10-01T11:00:01.000Z", set("line", 2))
	undated := historyRow("", set("line", 1))
	return []deduplicateCase{
		{name: "a timestamped line never matches an undated primary row",
			primary: rows(primaryRow("")), secondary: withTolerance(toleranceSeconds, historyRow(baseTime)),
			want: rows(historyRow(baseTime))},
		{name: "a timestamped line never matches an undated primary row with a wide window",
			primary: rows(primaryRow("")), secondary: withTolerance(3600, historyRow(baseTime)),
			want: rows(historyRow(baseTime))},
		{name: "undated rows on both sides match",
			primary: rows(primaryRow("")), secondary: withTolerance(toleranceSeconds, historyRow(""))},
		{name: "undated lines pair one to one, the dated primary row first",
			primary: rows(primaryRow("", set("line", 5)), primaryRow(baseTime, set("line", 3))),
			secondary: withTolerance(toleranceSeconds, historyRow("", set("line", 1)),
				historyRow("", set("line", 2)), historyRow("", set("line", 3))),
			want: rows(historyRow("", set("line", 3))), dropped: map[string]int{"claude-code": 2}},
		{name: "an undated line takes the earliest unused candidate",
			primary: rows(late, early), secondary: withTolerance(toleranceSeconds, undated, dated)},
		{name: "an undated line finds nothing once the dated line took the only row",
			primary: rows(late), secondary: withTolerance(toleranceSeconds, undated, dated),
			want: rows(undated)},
		{name: "an undated line never takes the row a timestamped line pairs with",
			primary: rows(primaryRow(baseTime)), secondary: withTolerance(toleranceSeconds,
				historyRow("", set("line", 1)), historyRow("2026-10-01T10:00:01.000Z",
					set("line", 2))),
			want: rows(historyRow("", set("line", 1)))},
	}
}

// toleranceCases are the tests of a tolerance other than the default,
// and of each line keeping its own.
func toleranceCases() []deduplicateCase {
	return []deduplicateCase{
		{name: "a ten-minute tolerance matches a line five minutes away",
			primary:   rows(primaryRow(baseTime)),
			secondary: withTolerance(600, historyRow("2026-10-01T10:05:00.000Z"))},
		{name: "the default tolerance keeps a line five minutes away",
			primary:   rows(primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, historyRow("2026-10-01T10:05:00.000Z")),
			want:      rows(historyRow("2026-10-01T10:05:00.000Z"))},
		{name: "each line keeps its own tolerance",
			primary: rows(primaryRow(baseTime), primaryRow(baseTime, set("text", "x"))),
			secondary: []Secondary{
				{historyRow("2026-10-01T10:05:00.000Z", set("line", 1)), 600},
				{historyRow("2026-10-01T10:05:00.000Z", set("line", 2), set("text", "x")), 2}},
			want:    rows(historyRow("2026-10-01T10:05:00.000Z", set("line", 2), set("text", "x"))),
			dropped: map[string]int{"claude-code": 1}},
	}
}

// copiesAmongSecondaryCases are the tests of copies among secondary rows
// that no primary row stands beside.
func copiesAmongSecondaryCases() []deduplicateCase {
	copyA := historyRow("", set("file", backupA))
	copyB := historyRow("", set("file", backupB))
	stampA := historyRow(baseTime, set("file", backupA))
	far := historyRow("2026-10-01T10:00:02.001Z", set("file", backupB))
	return []deduplicateCase{
		{name: "two undated backups of one history keep the first",
			secondary: withTolerance(toleranceSeconds, copyA, copyB), want: rows(copyA),
			dropped: map[string]int{"claude-code": 1}},
		{name: "timestamped copies within the tolerance keep the first",
			secondary: withTolerance(toleranceSeconds, stampA, historyRow("2026-10-01T10:00:01.500Z",
				set("file", backupB))), want: rows(stampA)},
		{name: "timestamped copies past the tolerance are both kept",
			secondary: withTolerance(toleranceSeconds, stampA, far), want: rows(stampA, far)},
		{name: "the tolerance among copies is that of the row tested",
			secondary: []Secondary{{stampA, 2}, {far, 600}}, want: rows(stampA)},
		{name: "the earlier copy is kept wherever it comes in the input",
			secondary: withTolerance(toleranceSeconds, historyRow("2026-10-01T10:00:01.000Z",
				set("file", backupA)), historyRow(baseTime, set("file", backupB))),
			want: rows(historyRow(baseTime, set("file", backupB)))},
	}
}

// copiesOfPrimaryAndParserCases are the tests of copies of a primary event,
// of a copy that takes a primary row, and of parser: rows.
func copiesOfPrimaryAndParserCases() []deduplicateCase {
	copyA := historyRow("", set("file", backupA))
	copyB := historyRow("", set("file", backupB))
	stampA := historyRow(baseTime, set("file", backupA))
	loose := historyRow("", set("session", ""), set("project", ""), set("file", backupA))
	parser := "parser: unreadable (Permission denied)"
	parserRow := historyRow("", set("turn", "system"), set("text", parser))
	parserBackup := historyRow("", set("turn", "system"), set("text", parser),
		set("file", backupA))
	return []deduplicateCase{
		{name: "three copies of a primary event collapse onto it and each other",
			primary: rows(primaryRow(baseTime)), secondary: withTolerance(toleranceSeconds, stampA,
				historyRow("2026-10-01T10:00:00.500Z", set("file", backupB)),
				historyRow("2026-10-01T10:00:01.000Z", set("file", backupA), set("line", 2))),
			dropped: map[string]int{"claude-code": 3}},
		{name: "undated copies of a primary event collapse",
			primary: rows(primaryRow(baseTime)), secondary: withTolerance(toleranceSeconds, copyA, copyB),
			dropped: map[string]int{"claude-code": 2}},
		{name: "undated copies of a primary event collapse in the other order",
			primary: rows(primaryRow(baseTime)), secondary: withTolerance(toleranceSeconds, copyB, copyA),
			dropped: map[string]int{"claude-code": 2}},
		{name: "an undated copy after a dated one is dropped",
			secondary: withTolerance(toleranceSeconds, stampA, copyB), want: rows(stampA),
			dropped: map[string]int{"claude-code": 1}},
		{name: "an undated copy before a dated one is dropped",
			secondary: withTolerance(toleranceSeconds, copyB, stampA), want: rows(stampA),
			dropped: map[string]int{"claude-code": 1}},
		{name: "the session-scoped undated copy takes the primary row",
			primary: rows(primaryRow(baseTime)), secondary: withTolerance(toleranceSeconds, loose, copyB),
			dropped: map[string]int{"claude-code": 2}},
		{name: "a copy with only a session meets one with only a project under its identity",
			primary: rows(primaryRow(baseTime)), secondary: withTolerance(toleranceSeconds,
				historyRow(baseTime, set("session", ""), set("file", backupA)),
				historyRow("2026-10-01T10:00:00.300Z", set("project", ""), set("file", backupB))),
			dropped: map[string]int{"claude-code": 2}},
		{name: "parser rows never pair",
			primary: rows(primaryRow("", set("turn", "system"), set("text", parser)),
				primaryRow(baseTime)),
			secondary: withTolerance(toleranceSeconds, parserRow, parserBackup, historyRow(baseTime)),
			want:      rows(parserRow, parserBackup), dropped: map[string]int{"claude-code": 1}},
	}
}

// copyCases are the tests of copies among secondary rows, of tool
// rows paired on their id and of parser: rows.
func copyCases() []deduplicateCase {
	cases := slices.Concat(copiesAmongSecondaryCases(), copiesOfPrimaryAndParserCases(),
		broaderCases(), repeatCases())
	for _, turnType := range []string{timeline.TurnToolUse, timeline.TurnToolResult} {
		cases = append(cases, toolCases(turnType)...)
	}
	return cases
}

// broaderCases test that a broader scope among copies never
// takes a candidate of another session or project, and that the primary
// pass stays strict.
func broaderCases() []deduplicateCase {
	sessionA := historyRow(baseTime, set("text", "yes"), set("file", backupA))
	sessionB := historyRow(baseTime, set("text", "yes"), set("session", otherID),
		set("file", backupB))
	projectA := historyRow(baseTime, set("text", "yes"), set("session", ""),
		set("file", backupA))
	projectB := historyRow(baseTime, set("text", "yes"), set("project", "/srv/other"),
		set("file", backupB))
	scoped := historyRow(baseTime, set("session", otherID), set("project", ""))
	return []deduplicateCase{
		{name: "copies in two sessions are two events",
			secondary: withTolerance(toleranceSeconds, sessionA, sessionB), want: rows(sessionA, sessionB)},
		{name: "copies in two projects are two events",
			secondary: withTolerance(toleranceSeconds, projectA, projectB), want: rows(projectA, projectB)},
		{name: "a row with a session never takes a primary row without one",
			primary:   rows(primaryRow(baseTime, set("session", ""))),
			secondary: withTolerance(toleranceSeconds, scoped), want: rows(scoped)},
	}
}

// repeatCases are the tests of repeats inside one source and of
// chained copies.
func repeatCases() []deduplicateCase {
	minute := "2026-10-01T10:01:00.000Z"
	firstA := historyRow(baseTime, set("text", "continue"), set("file", backupA))
	secondA := historyRow(minute, set("text", "continue"), set("line", 2), set("file", backupA))
	firstB := historyRow(baseTime, set("text", "continue"), set("file", backupB))
	secondB := historyRow(minute, set("text", "continue"), set("line", 2), set("file", backupB))
	yesA := historyRow("", set("text", "yes"), set("file", backupA))
	yesA3 := historyRow("", set("text", "yes"), set("line", 3), set("file", backupA))
	yesB := historyRow("", set("text", "yes"), set("file", backupB))
	stamped := rows(historyRow(baseTime, set("text", "yes"), set("file", backupA)),
		historyRow(baseTime, set("text", "yes"), set("line", 2), set("file", backupA)))
	return []deduplicateCase{
		{name: "two backups with two repeats each keep two rows",
			secondary: withTolerance(toleranceSeconds, firstA, secondA, firstB, secondB),
			want:      rows(firstA, secondA)},
		{name: "three undated copies keep one",
			secondary: withTolerance(toleranceSeconds, historyRow("", set("file", backupA)),
				historyRow("", set("file", backupB)), historyRow("", set("file", backupC))),
			want:    rows(historyRow("", set("file", backupA))),
			dropped: map[string]int{"claude-code": 2}},
		{name: "two repeats in one undated source both stay",
			secondary: withTolerance(toleranceSeconds, yesA, yesA3), want: rows(yesA, yesA3),
			dropped: map[string]int{}},
		{name: "the same line in two files is one event",
			secondary: withTolerance(toleranceSeconds, yesA, yesB), want: rows(yesA)},
		{name: "two timestamped repeats in one source both stay",
			secondary: withTolerance(toleranceSeconds, stamped...), want: stamped},
	}
}

// toolCases are the tests of tool rows paired on their tool_use_id,
// for one turn type.
func toolCases(turnType string) []deduplicateCase {
	command := `{"cmd": ["ls", "-la"]}`
	transcript := primaryRow(baseTime, set("turn", turnType), set("id", "call_1"),
		set("text", command))
	differentID := historyRow(baseTime, set("turn", turnType), set("id", "call_2"),
		set("text", command))
	otherText := historyRow(baseTime, set("turn", turnType), set("text", "ls"))
	userRow := historyRow(baseTime, set("id", "call_1"), set("text", "other"))
	copyA := historyRow(baseTime, set("turn", turnType), set("id", "call_1"), set("text", "a"),
		set("file", backupA))
	copyB := historyRow(baseTime, set("turn", turnType), set("id", "call_1"), set("text", "b"),
		set("file", backupB))
	return []deduplicateCase{
		{name: turnType + " rows with the same id pair whatever their text",
			primary: rows(transcript), secondary: withTolerance(toleranceSeconds, historyRow(baseTime,
				set("turn", turnType), set("id", "call_1"), set("text", "ls -la"))),
			dropped: map[string]int{"claude-code": 1}},
		{name: turnType + " rows with other ids never pair",
			primary: rows(transcript), secondary: withTolerance(toleranceSeconds, differentID),
			want: rows(differentID)},
		{name: turnType + " row without an id pairs by text with one that has it",
			primary: rows(transcript), secondary: withTolerance(toleranceSeconds, historyRow(baseTime,
				set("turn", turnType), set("text", command)))},
		{name: turnType + " row without an id and other text is kept",
			primary: rows(transcript), secondary: withTolerance(toleranceSeconds, otherText),
			want: rows(otherText)},
		{name: turnType + " rows without ids pair by text",
			primary: rows(primaryRow(baseTime, set("turn", turnType), set("text", "ls -la"))),
			secondary: withTolerance(toleranceSeconds, historyRow(baseTime, set("turn", turnType),
				set("text", "ls -la")))},
		{name: turnType + " case: the id never stands in for a user row's text",
			primary:   rows(primaryRow(baseTime, set("id", "call_1"))),
			secondary: withTolerance(toleranceSeconds, userRow), want: rows(userRow)},
		{name: turnType + " copies among secondary rows pair on the id",
			secondary: withTolerance(toleranceSeconds, copyA, copyB), want: rows(copyA)},
	}
}
