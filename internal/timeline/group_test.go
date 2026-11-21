package timeline

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The times of a group's two parents and of a member with a time of its
// own, and a bound between the parents.
const (
	oldTime    = "2026-01-01T00:00:00.000Z"
	newTime    = "2026-06-01T00:00:00.000Z"
	ownTime    = "2026-06-02T00:00:00.000Z"
	betweenOld = "2026-03-01T00:00:00.000Z"
)

// filter is what a test keeps of rows: those inside since and until ("" for
// no bound; an undated row is kept) whose text matches pattern, when set.
type filter struct {
	since, until string
	pattern      *regexp.Regexp
}

// keep returns the rows the filter keeps, in order.
func (f filter) keep(rows []Row) []Row {
	var kept []Row
	for _, row := range rows {
		dated := row.TimestampUTC != ""
		if dated && (f.since != "" && row.TimestampUTC < f.since || f.until != "" && row.TimestampUTC > f.until) {
			continue
		}
		if f.pattern == nil || row.Matches(f.pattern) {
			kept = append(kept, row)
		}
	}
	return kept
}

// tested returns what the filter tests of a row.
func (f filter) tested() Tested {
	var tested Tested
	if f.since != "" || f.until != "" {
		tested |= TestedTime
	}
	if f.pattern != nil {
		tested |= TestedText
	}
	return tested
}

// settle chooses each group's parent from what the filter keeps of rows,
// attaches a copy of rows, and returns what the filter then keeps of it,
// sorted, as the timeline command does; a nil filter is no filter.
func settle(rows []Row, rowFilter *filter) []Row {
	var choice Choice
	if rowFilter != nil {
		choice = Choose(rowFilter.keep(rows), rowFilter.tested())
	}
	attached := Attach(slices.Clone(rows), choice)
	SortRows(attached)
	if rowFilter == nil {
		return attached
	}
	return rowFilter.keep(attached)
}

// summary returns each row as "time session project branch model text".
func summary(rows []Row) []string {
	var lines []string
	for _, row := range rows {
		lines = append(lines, strings.Join([]string{row.TimestampUTC, row.SessionID, row.ProjectPath, row.GitBranch,
			row.Model, row.Text}, " "))
	}
	return lines
}

// parentRow returns a parent's own row.
func parentRow(timestamp, session, name, text string) Row {
	return Row{TimestampUTC: timestamp, SessionID: session, ProjectPath: "/" + name, GitBranch: "branch-" + name,
		Model: "model-" + name, Text: text, SourceFile: "f", SourceLine: 1}
}

// memberRows returns a group of two parents, old and new, and two members
// that take session, project, branch and model, one with a time of its own
// and one taking the parent's time.
func memberRows() []Row {
	group := NewGroup()
	old, current := parentRow(oldTime, "s-old", "old", "old start"), parentRow(newTime, "s-new", "new", "new start")
	group.Parent(&old)
	group.Parent(&current)
	dated := Row{TimestampUTC: ownTime, Text: "dated member", SourceFile: "f", SourceLine: 2}
	undated := Row{Text: "undated member", SourceFile: "f", SourceLine: 3}
	takes := TakeSession | TakeProject | TakeBranch | TakeModel
	group.Member(&dated, takes)
	group.Member(&undated, takes|TakeTime)
	return []Row{old, dated, undated, current}
}

func TestMembersTakeTheirAttributesFromTheFirstParentTheFilterKeeps(t *testing.T) {
	old := oldTime + " s-old /old branch-old model-old "
	current := newTime + " s-new /new branch-new model-new "
	withOld := ownTime + " s-old /old branch-old model-old dated member"
	withNew := ownTime + " s-new /new branch-new model-new dated member"
	tests := []struct {
		name   string
		filter *filter
		want   []string
	}{
		{"without a filter the group goes with its first parent", nil,
			[]string{old + "old start", old + "undated member", current + "new start", withOld}},
		{"a window past the first parent moves the members to the second", &filter{since: betweenOld},
			[]string{current + "new start", current + "undated member", withNew}},
		{"a window before the second parent keeps them with the first", &filter{until: betweenOld},
			[]string{old + "old start", old + "undated member"}},
		{"a pattern only the second parent and the members match", &filter{pattern: regexp.MustCompile("new|member")},
			[]string{current + "new start", current + "undated member", withNew}},
		{"a filter that keeps no parent leaves the group with its first", &filter{pattern: regexp.MustCompile("member")},
			[]string{old + "undated member", withOld}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := summary(settle(memberRows(), test.filter)); !slices.Equal(got, test.want) {
				t.Errorf("got  %q\nwant %q", got, test.want)
			}
		})
	}
}

// A member is given the first parent's attributes as it joins, so the
// rows a parse gives read as the output without a filter.
func TestAMemberReadsAsWithoutAFilterBeforeAttaching(t *testing.T) {
	rows := memberRows()
	if got, want := summary(rows[1:3]), []string{ownTime + " s-old /old branch-old model-old dated member",
		oldTime + " s-old /old branch-old model-old undated member"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// textRows returns a group of two parents whose texts depend on which the
// group goes with: chosen gives the text a parent shows when chosen from
// its own text, as an attachment added to the parent's text or a history
// text that replaces it does.
func textRows(chosen func(own string) string) []Row {
	group := NewGroup()
	var rows []Row
	for _, parent := range []struct{ timestamp, name, own string }{{oldTime, "old", "old own"},
		{newTime, "new", "new own"}} {
		row := parentRow(parent.timestamp, "s", parent.name, "")
		handle := group.Parent(&row)
		handle.Texts(&row, RowText{Text: chosen(parent.own)}, RowText{Text: parent.own})
		rows = append(rows, row)
	}
	return rows
}

func TestAGroupTextIsShownInTheFirstParentTheFilterKeeps(t *testing.T) {
	appended := func(own string) string { return own + "\n[attachment: a.txt]" }
	replaced := func(string) string { return "HISTORY TEXT" }
	tests := []struct {
		name   string
		chosen func(string) string
		filter *filter
		want   []string
	}{
		{"an attachment without a filter stays with the first parent", appended, nil,
			[]string{"old own\n[attachment: a.txt]", "new own"}},
		{"an attachment moves to the second parent past a window", appended, &filter{since: betweenOld},
			[]string{"new own\n[attachment: a.txt]"}},
		{"a pattern only the attachment matches finds it in the second parent too", appended,
			&filter{since: betweenOld, pattern: regexp.MustCompile("attachment")}, []string{"new own\n[attachment: a.txt]"}},
		{"a history text replaces the first parent's text without a filter", replaced, nil,
			[]string{"HISTORY TEXT", "new own"}},
		{"a pattern matching a later parent's own text shows the history there", replaced,
			&filter{pattern: regexp.MustCompile("new own")}, []string{"HISTORY TEXT"}},
		{"a pattern matching the history keeps it in the first parent", replaced,
			&filter{pattern: regexp.MustCompile("HISTORY")}, []string{"HISTORY TEXT"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for _, row := range settle(textRows(test.chosen), test.filter) {
				got = append(got, row.Text)
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

// Once attached, the chosen parent's row is matched against both its
// texts and another parent's row only against the text it shows.
func TestOnlyTheChosenParentIsMatchedAgainstBothTexts(t *testing.T) {
	rows := Attach(textRows(func(string) string { return "HISTORY TEXT" }), nil)
	own := regexp.MustCompile("own")
	if !rows[0].Matches(own) || !rows[0].Matches(regexp.MustCompile("HISTORY")) || !rows[1].Matches(own) ||
		rows[1].Matches(regexp.MustCompile("HISTORY")) {
		t.Errorf("rows %q: wrong matches", summary(rows))
	}
}

// replacedRows returns a group of two archived copies of one record: each
// parent gives its own row, shown when the group does not go with it, and
// the row of the thread linked to the record, which replaces it when the
// group does.
func replacedRows() []Row {
	group := NewGroup()
	var rows []Row
	for _, path := range []string{"/w/old", "/w/new"} {
		own := Row{TimestampUTC: oldTime, Text: "worktree " + path, SourceFile: "f"}
		handle := group.Parent(&own)
		handle.Row(&own, WhenNotChosen)
		thread := Row{TimestampUTC: oldTime, SessionID: "t1", Text: "worktree " + path + " of thread t1",
			SourceFile: "f"}
		handle.Row(&thread, WhenChosen)
		rows = append(rows, own, thread)
	}
	return rows
}

func TestTheChosenParentsRowIsReplacedByTheGroupsRows(t *testing.T) {
	tests := []struct {
		name   string
		filter *filter
		want   []string
	}{
		{"without a filter the first parent's row is replaced", nil,
			[]string{"worktree /w/old of thread t1", "worktree /w/new"}},
		{"a pattern the second parent's own row matches keeps that row shown, as the row replacing it is another",
			&filter{pattern: regexp.MustCompile("/w/new")}, []string{"worktree /w/new"}},
		{"a pattern only the second parent's replacing row matches replaces its row",
			&filter{pattern: regexp.MustCompile("of thread")}, []string{"worktree /w/old of thread t1"}},
		{"a pattern the first parent matches keeps the thread with it",
			&filter{pattern: regexp.MustCompile("worktree")}, []string{"worktree /w/old of thread t1", "worktree /w/new"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for _, row := range settle(replacedRows(), test.filter) {
				got = append(got, row.Text)
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

// A row copied from a parent's row is a row of the same parent, so the
// parent counts as kept when the filter keeps only the copy, and comes
// before a later parent the filter keeps.
func TestACopyOfAParentsRowCountsForItsParent(t *testing.T) {
	group := NewGroup()
	old, current := parentRow(oldTime, "s", "old", "old start"), parentRow(newTime, "s", "new", "new start")
	group.Parent(&old)
	copied := old
	copied.Text = "old setting"
	group.Parent(&current)
	member := Row{TimestampUTC: ownTime, Text: "member"}
	group.Member(&member, TakeProject)
	rows := settle([]Row{old, copied, current, member}, &filter{pattern: regexp.MustCompile("setting|member|new")})
	if got, want := summary(rows), []string{oldTime + " s /old branch-old model-old old setting",
		newTime + " s /new branch-new model-new new start", ownTime + "  /old   member"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// groupedRows returns n groups of two parents and two members each, beside
// n rows of no group.
func groupedRows(n int) []Row {
	var rows []Row
	for range n {
		rows = append(rows, memberRows()...)
		rows = append(rows, Row{TimestampUTC: ownTime, Text: "alone"})
	}
	return rows
}

// Choosing and attaching look at each row once and place each grouped
// row once, so their work doubles exactly with the rows.
func TestAttachingGrowsLinearlyWithTheRows(t *testing.T) {
	work := func(n int) int {
		rows := groupedRows(n)
		choice, chosen := choose(rows, TestedText)
		_, attached := attach(rows, choice)
		return chosen + attached
	}
	if small, large := work(500), work(1000); large != 2*small {
		t.Errorf("work %d for 500 groups and %d for 1000, want twice", small, large)
	}
}

// linkedRows returns a group of two stored copies of one record, each with
// its own row shown only when the group does not go with it, and two
// links of the record as members taking their project from the parent the
// group goes with; takes adds to what the links take. The new
// parent's text names a shared text.
func linkedRows(takes Takes) []Row {
	group := NewGroup()
	var rows []Row
	for _, path := range []string{"/w/old", "/w/new"} {
		var builder TextBuilder
		builder.Write("worktree " + path)
		if path == "/w/new" {
			builder.Write(" notes ")
			builder.Share(NewSharedText("NOTES"), "NOTES", func(*Row, bool) string { return "(notes above)" })
		}
		own := Row{TimestampUTC: oldTime, ProjectPath: path, SourceFile: "f"}
		builder.Set(&own)
		group.Parent(&own).Row(&own, WhenNotChosen)
		rows = append(rows, own)
	}
	for _, session := range []string{"t1", "t2"} {
		link := Row{TimestampUTC: oldTime, SessionID: session, Text: "link " + session, SourceFile: "f"}
		group.Member(&link, takes|TakeProject)
		rows = append(rows, link)
	}
	return rows
}

// linkSummary returns each row as "session project text", the text as it
// reads in full, and whether it names a shared text.
func linkSummary(rows []Row) []string {
	var lines []string
	for _, row := range rows {
		shared := ""
		if row.Shared != nil {
			shared = " (shared)"
		}
		lines = append(lines, row.SessionID+" "+row.ProjectPath+" "+row.FullText()+shared)
	}
	return lines
}

func TestAMemberTakingTextShowsTheTextOfTheParentTheGroupGoesWith(t *testing.T) {
	newText := "worktree /w/new notes NOTES (shared)"
	tests := []struct {
		name   string
		takes  Takes
		filter *filter
		want   []string
	}{
		{"without a filter the links show the first parent's text", TakeText, nil,
			[]string{" /w/new " + newText, "t1 /w/old worktree /w/old", "t2 /w/old worktree /w/old"}},
		{"a pattern only the second parent matches gives the links its text and shared texts", TakeText,
			&filter{pattern: regexp.MustCompile("/w/new")}, []string{"t1 /w/new " + newText, "t2 /w/new " + newText}},
		{"a pattern only the first parent matches keeps the links with it", TakeText,
			&filter{pattern: regexp.MustCompile("/w/old")}, []string{"t1 /w/old worktree /w/old",
				"t2 /w/old worktree /w/old"}},
		{"a pattern neither parent matches shows nothing", TakeText,
			&filter{pattern: regexp.MustCompile("absent")}, nil},
		{"a link not taking text leaves a matched later parent's own row shown", 0,
			&filter{pattern: regexp.MustCompile("/w/new|link")},
			[]string{" /w/new " + newText, "t1 /w/old link t1", "t2 /w/old link t2"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := linkSummary(settle(linkedRows(test.takes), test.filter)); !slices.Equal(got, test.want) {
				t.Errorf("got  %q\nwant %q", got, test.want)
			}
		})
	}
}

// A member taking text is given the first parent's text as it joins, so
// the rows a parse gives read as the output without a filter.
func TestAMemberTakingTextReadsAsWithoutAFilterBeforeAttaching(t *testing.T) {
	rows := linkedRows(TakeText)
	if got, want := linkSummary(rows[2:]), []string{"t1 /w/old worktree /w/old",
		"t2 /w/old worktree /w/old"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// sourceLines returns what a filter keeps of a group of two stored copies
// of one record, at source lines 1 and 2, each with its own row shown only
// when the group does not go with it, and two links of the record at the
// first copy's line, as a join gives them, taking its text and what takes
// adds from the copy the group goes with. Each row reads "source line
// text", sorted.
func sourceLines(takes Takes, rowFilter *filter) []string {
	group := NewGroup()
	var rows []Row
	for line, path := range []string{"/w/old", "/w/new"} {
		own := Row{TimestampUTC: oldTime, Text: "worktree " + path, SourceFile: "f", SourceLine: line + 1}
		group.Parent(&own).Row(&own, WhenNotChosen)
		rows = append(rows, own)
	}
	for range 2 {
		link := Row{TimestampUTC: oldTime, SourceFile: "f", SourceLine: 1}
		group.Member(&link, takes|TakeText)
		rows = append(rows, link)
	}
	var lines []string
	for _, row := range settle(rows, rowFilter) {
		lines = append(lines, strconv.Itoa(row.SourceLine)+" "+row.Text)
	}
	slices.Sort(lines)
	return lines
}

func TestAMemberTakingTheSourceLineNamesTheParentTheGroupGoesWith(t *testing.T) {
	newPattern := &filter{pattern: regexp.MustCompile("/w/new")}
	tests := []struct {
		name   string
		takes  Takes
		filter *filter
		want   []string
	}{
		{"without a filter the links name the first parent's line", TakeSourceLine, nil,
			[]string{"1 worktree /w/old", "1 worktree /w/old", "2 worktree /w/new"}},
		{"a pattern only the second parent matches gives the links its line with its text", TakeSourceLine,
			newPattern, []string{"2 worktree /w/new", "2 worktree /w/new"}},
		{"a link not taking the source line keeps its own beside the second parent's text", 0, newPattern,
			[]string{"1 worktree /w/new", "1 worktree /w/new"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sourceLines(test.takes, test.filter); !slices.Equal(got, test.want) {
				t.Errorf("got  %q\nwant %q", got, test.want)
			}
		})
	}
}

// contentRows returns a group of two stored copies of one message, each
// with its notice (two texts), its model row, and its own content, shown
// only when the group does not go with it since the message's part, a
// member with no time of its own, is shown in its place with other text.
func contentRows() []Row {
	group := NewGroup()
	var rows []Row
	for _, parent := range []struct{ timestamp, name string }{{oldTime, "old"}, {newTime, "new"}} {
		notice := parentRow(parent.timestamp, "s-"+parent.name, parent.name, "")
		handle := group.Parent(&notice)
		handle.Texts(&notice, RowText{Text: "notice: parts with this row"},
			RowText{Text: "notice: parts with another"})
		model := parentRow(parent.timestamp, "s-"+parent.name, parent.name, "model "+parent.name)
		handle.Row(&model, Always)
		content := parentRow(parent.timestamp, "s-"+parent.name, parent.name, "content "+parent.name)
		handle.Row(&content, WhenNotChosen)
		rows = append(rows, notice, model, content)
	}
	part := Row{Text: "part text", SourceFile: "f", SourceLine: 9}
	group.Member(&part, TakeTime|TakeSession|TakeModel)
	return append(rows, part)
}

// movedRows returns a group of two stored copies of one message, the
// later with a model row shown only when the group goes with it, and a
// later message's model row of that parent shown only when it does not:
// the first row names the change it reads when it is, or another change
// when other is set.
func movedRows(other bool) []Row {
	group := NewGroup()
	old, current := parentRow(oldTime, "s", "old", "old answer"), parentRow(newTime, "s", "new", "")
	group.Parent(&old)
	handle := group.Parent(&current)
	moved := parentRow(newTime, "s", "new", "model: a -> b")
	handle.Row(&moved, WhenChosen)
	if other {
		moved.Text = "model: a -> c"
	}
	call := Row{Text: "the call", SourceFile: "f", SourceLine: 7}
	group.Member(&call, TakeTime|TakeModel)
	later := parentRow(ownTime, "s", "later", "model: a -> b")
	handle.Row(&later, WhenNotChosen)
	return []Row{old, call, moved, later}
}

// tagged returns rows with each row's tool_name set to its index, which
// no grouped row takes, so a row of the output names the row it shows.
func tagged(rows []Row) []Row {
	for i := range rows {
		rows[i].ToolName = "row " + strconv.Itoa(i)
	}
	return rows
}

// groupTexts returns every text of rows: each row's own and, for a row
// with two texts, both.
func groupTexts(rows []Row) []string {
	var texts []string
	for _, row := range rows {
		texts = append(texts, row.FullText())
		if row.Group != nil && row.Group.texts != nil {
			texts = append(texts, row.Group.texts[0].Text, row.Group.texts[1].Text)
		}
	}
	return texts
}

// keepsWhatItMatches returns what is wrong with the output of rows under
// a filter of pattern: a row shown twice, or a row of the output without
// a filter that the pattern matches and the filtered output does not show,
// with its text or, for a row with two texts, with its chosen one.
func keepsWhatItMatches(build func() []Row, pattern *regexp.Regexp) []string {
	unfiltered := settle(tagged(build()), nil)
	filtered := settle(tagged(build()), &filter{pattern: pattern})
	var faults []string
	seen := map[string]bool{}
	for _, row := range filtered {
		if seen[row.ToolName] {
			faults = append(faults, row.ToolName+" shown twice")
		}
		seen[row.ToolName] = true
	}
	for _, hit := range unfiltered {
		if !hit.Matches(pattern) {
			continue
		}
		shown := slices.ContainsFunc(filtered, func(row Row) bool {
			chosen := row.ToolName == hit.ToolName && row.Group != nil && row.Group.texts != nil &&
				row.Text == row.Group.texts[0].Text
			return row.FullText() == hit.FullText() || chosen
		})
		if !shown {
			faults = append(faults, "hides "+strconv.Quote(hit.FullText()))
		}
	}
	return faults
}

// Whatever pattern a filter matches, every row of the output without a
// filter that it matches is shown, with its text or, for a row with two
// texts, the chosen one, and no row is shown twice: a parent is never
// chosen by, nor kept, a row it would then hide with nothing showing its
// text in its place. Each pattern is one text of the group's rows, or two.
func TestAFilterShowsEveryRowItMatchesOfTheOutputWithoutIt(t *testing.T) {
	shapes := []struct {
		name  string
		build func() []Row
	}{
		{"members taking a parent's attributes", memberRows},
		{"an attachment added to the chosen parent's text",
			func() []Row { return textRows(func(own string) string { return own + "\n[attachment: a.txt]" }) }},
		{"a history text replacing the chosen parent's text",
			func() []Row { return textRows(func(string) string { return "HISTORY TEXT" }) }},
		{"a parent's own row replaced by a row with other text", replacedRows},
		{"links taking the text of a parent's own row", func() []Row { return linkedRows(TakeText) }},
		{"links not taking the text of a parent's own row", func() []Row { return linkedRows(0) }},
		{"a message's content replaced by its part", contentRows},
		{"a later model row replaced by the same change", func() []Row { return movedRows(false) }},
		{"a later model row replaced by another change", func() []Row { return movedRows(true) }},
	}
	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			texts := groupTexts(shape.build())
			for _, first := range texts {
				for _, second := range texts {
					pattern := regexp.MustCompile(regexp.QuoteMeta(first) + "|" + regexp.QuoteMeta(second))
					if faults := keepsWhatItMatches(shape.build, pattern); len(faults) > 0 {
						t.Errorf("pattern %q: %q", pattern, faults)
					}
				}
			}
		})
	}
}

func TestARowShownOnlyWhenChosenIsShownWhenItsParentIsChosen(t *testing.T) {
	tests := []struct {
		name    string
		other   bool
		pattern string
		want    []string
	}{
		{"a pattern only that row and its same change match moves the call to its parent", false,
			"-> b|the call", []string{"model: a -> b", "the call"}},
		{"a pattern only that row matches shows it", true, "-> c", []string{"model: a -> c"}},
		{"a pattern that also matches a later row it would hide passes its parent over", true,
			"-> c|-> b", []string{"model: a -> b"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for _, row := range settle(movedRows(test.other), &filter{pattern: regexp.MustCompile(test.pattern)}) {
				got = append(got, row.Text)
			}
			slices.Sort(got)
			if !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestAHiddenRowCountsOnlyWhenWhatReplacesItCarriesWhatTheFilterTested(t *testing.T) {
	tests := []struct {
		name   string
		filter *filter
		want   []string
	}{
		{"a pattern only the later content matches keeps it shown", &filter{pattern: regexp.MustCompile("content new")},
			[]string{"content new"}},
		{"a pattern also matching the later model row keeps the content shown",
			&filter{pattern: regexp.MustCompile("content new|model new")}, []string{"model new", "content new"}},
		{"a window that keeps only the later copy moves the part there, its time the content's",
			&filter{since: betweenOld}, []string{"notice: parts with this row", "model new", "part text"}},
		{"a window and a pattern on the later content keep it shown",
			&filter{since: betweenOld, pattern: regexp.MustCompile("content new")}, []string{"content new"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for _, row := range settle(contentRows(), test.filter) {
				got = append(got, row.Text)
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
