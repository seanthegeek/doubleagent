package analyze

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// Kilo Code's CLI keeps OpenCode's databases under its own directory, read
// by the same reader, so each shape of repeated id the OpenCode tests
// build is shown the same way at Kilo's path.

// kilocodeDatabase is where Kilo Code's CLI keeps its database.
const kilocodeDatabase = ".local/share/kilo/kilo.db"

// kilocodeRun runs timeline over a collection holding the database given
// at Kilo's path, with the options given, and returns each row of
// timeline.jsonl as "time session project text".
func kilocodeRun(t *testing.T, data string, options ...string) []string {
	t.Helper()
	return opencodeDatabaseRun(t, "kilo-code", []collectedFile{{"alice", "/home/alice", "kilo-code",
		kilocodeDatabase, data}}, options...)
}

func TestARepeatedKiloCodeMessageInARepeatedSessionKeepsTheFirstSessionRowsProject(t *testing.T) {
	rows := kilocodeRun(t, opencodeDoubleRepeat(t), "--since", "2026-03-01")
	message := "parser: session_message holds 2 rows of this row's id; its parts are shown once, with "
	want := []string{sessionStartD, sessionRowD + sessionShownV2 + "this row",
		sessionMembers + "ses_y /proj/d V2-TEXT", sessionMembers + "ses_y /proj/c " + message + "this row",
		sessionMembers + "ses_y /proj/c model: m provider=p", sessionMembers + "ses_y /proj/c ITEM-TEXT",
		sessionMembers + "ses_y /proj/c " + message + "another of them", sessionMembers + "ses_y /proj/c OWN"}
	if !slices.Equal(rows, want) {
		t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(want, "\n"))
	}
}

func TestAKiloCodeMessagesOwnDirectoryStaysWhenTheFirstSessionRowNamesNone(t *testing.T) {
	for _, test := range []opencodeRepeatCase{
		{"with a pattern both rows' start rows match", nil, []string{
			"2026-01-01T00:00:00.000Z ses_x  session start: A", sessionStartB,
			"2026-06-02T00:00:00.000Z ses_x /own CWD-TEXT", "2026-06-02T00:00:01.000Z ses_x  BARE-TEXT"}},
		{"with a window past the first row", []string{"--since", "2026-03-01"}, []string{sessionStartB,
			"2026-06-02T00:00:00.000Z ses_x /own CWD-TEXT", "2026-06-02T00:00:01.000Z ses_x /proj/b BARE-TEXT"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := kilocodeRun(t, opencodeUnplacedRepeat(t), append([]string{"--match", "session start|-TEXT"},
				test.options...)...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

func TestTheProblemsOfARepeatedKiloCodeSessionTakeTheirOwnersProject(t *testing.T) {
	message := " ses_x %s parser: unparseable record 1 (message.data: invalid JSON at line 1, column 1: " +
		"invalid character 'M' looking for beginning of value): MESSAGE"
	permission := " ses_x /proj/b parser: unparseable record 2 (session.permission: not a JSON array): RULES"
	for _, test := range []opencodeRepeatCase{
		{"a pattern only the record's problem matches keeps it with the first row", []string{"--match", "MESSAGE"},
			[]string{fmt.Sprintf(message, "/proj/a")}},
		{"a pattern the second row's problem matches moves it to the second", []string{"--match", "unparseable"},
			[]string{fmt.Sprintf(message, "/proj/b"), permission}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := kilocodeRun(t, opencodeDamagedRepeat(t), test.options...)
			slices.Sort(rows)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}
