package analyze

import (
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/rowfilter"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// linkedStores returns the stores whose repeated key is a link other rows
// take their project path through: a Copilot Desktop project, workspace
// or session alias, and the two links at once, and an OpenCode project,
// alone and with a repeated session of it.
func linkedStores() []groupedStore {
	file := func(build func(t *testing.T) string) func(t *testing.T) []collectedFile {
		return func(t *testing.T) []collectedFile {
			return []collectedFile{{"alice", "/home/alice", "copilot-desktop", ".copilot/data.db", build(t)}}
		}
	}
	return []groupedStore{
		{"copilot desktop projects", "copilot-desktop", file(copilotdesktopRepeatedProject)},
		{"copilot desktop workspaces", "copilot-desktop", file(copilotdesktopRepeatedWorkspace)},
		{"copilot desktop aliases", "copilot-desktop", file(copilotdesktopRepeatedAlias)},
		{"copilot desktop chain", "copilot-desktop", file(copilotdesktopRepeatedChain)},
		{"opencode projects", "opencode", func(t *testing.T) []collectedFile {
			return []collectedFile{{"alice", "/home/alice", "opencode", ".local/share/opencode/opencode.db",
				opencodeProjectRepeat(t)}}
		}},
		{"opencode projects and sessions", "opencode", func(t *testing.T) []collectedFile {
			return []collectedFile{{"alice", "/home/alice", "opencode", ".local/share/opencode/opencode.db",
				opencodeProjectAndSessionRepeat(t)}}
		}},
	}
}

// checkWindows checks that a window from or to the time of any row of the
// store's output without a filter shows every dated row of that output
// inside it, and no row more often than that output does.
func checkWindows(t *testing.T, directory, agent string) {
	t.Helper()
	unfiltered := timelineRows(t, directory, agent, rowfilter.RowFilter{}, nil, false)
	var times []string
	for _, row := range unfiltered {
		if row.TimestampUTC != "" && !slices.Contains(times, row.TimestampUTC) {
			times = append(times, row.TimestampUTC)
		}
	}
	for _, bound := range times {
		for _, window := range []rowfilter.RowFilter{{Since: bound}, {Until: bound}} {
			filtered := timelineRows(t, directory, agent, window, nil, false)
			if faults := keepFaults(unfiltered, filtered, func(row *timeline.Row) bool {
				return row.TimestampUTC != "" && (window.Since == "" || row.TimestampUTC >= window.Since) &&
					(window.Until == "" || row.TimestampUTC <= window.Until)
			}); len(faults) > 0 {
				t.Errorf("--since %q --until %q: %q", window.Since, window.Until, faults)
			}
		}
	}
}

// checkMatches checks that a pattern made of the texts of one or two of
// the grouped rows the parser gave shows every row of the output without a
// filter it matches, and no row more often than that output does.
func checkMatches(t *testing.T, directory, agent string) {
	t.Helper()
	var texts []string
	for _, row := range timelineRows(t, directory, agent, rowfilter.RowFilter{}, nil, true) {
		if text := row.FullText(); text != "" && !slices.Contains(texts, regexp.QuoteMeta(text)) {
			texts = append(texts, regexp.QuoteMeta(text))
		}
	}
	if len(texts) == 0 {
		t.Fatal("the store gave no grouped row")
	}
	unfiltered := timelineRows(t, directory, agent, rowfilter.RowFilter{}, nil, false)
	for i, first := range texts {
		for _, second := range texts[i:] {
			pattern := first + "|" + second
			filtered := timelineRows(t, directory, agent, rowfilter.RowFilter{}, []string{pattern}, false)
			matched := regexp.MustCompile(pattern)
			if faults := keepFaults(unfiltered, filtered, func(row *timeline.Row) bool {
				return row.Matches(matched)
			}); len(faults) > 0 {
				t.Errorf("--match %q: %q", pattern, faults)
			}
		}
	}
}

// For every store whose repeated key is a link, a window or a pattern
// keeps every row of the output without a filter that it matches, and
// shows no row more often than that output does.
func TestAFilterShowsEveryRowOfARepeatedLinkItMatches(t *testing.T) {
	for _, store := range linkedStores() {
		t.Run(store.name, func(t *testing.T) {
			directory := writeCollection(t, filepath.Join(t.TempDir(), "col"),
				[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, store.files(t))
			checkWindows(t, directory, store.agent)
			checkMatches(t, directory, store.agent)
		})
	}
}
