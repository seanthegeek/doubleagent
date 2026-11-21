package timeline

// Group joins the rows of an id an agent's store repeats, which only damage
// or an edit writes: each stored row of the id is a parent, and the rows of
// the id itself are members, shown once with what they take from the parent
// the group goes with: its first, in the order the parser registered them,
// that the --since, --until and --match filter keeps (Choose), else its
// first. Attach settles that once the output's rows are known, so a planted
// earlier row with a real record's id cannot move the record's rows out of
// a filter's window or give them its attributes. See "Rows of a repeated
// id" in analyzer/docs/output.md.
type Group struct {
	parents []Row
}

// NewGroup returns a group with no parent yet. A parser makes one per
// repeated id in one file.
func NewGroup() *Group {
	return &Group{}
}

// Parent is one parent of a group, the handle its rows are registered by.
type Parent struct {
	group *Group
	index int
}

// Takes says what a member row takes from the parent its group goes with.
type Takes uint8

// The attributes a member can take. TakeTime is for a member whose time
// is the parent's because it has none of its own. TakeText is for a member
// whose text is the parent's row's, with its shared texts, as that row
// held them when the parent was registered. TakeSourceLine is for a member
// whose source_line names the stored row its group goes with, as a member
// whose text is that row's does.
const (
	TakeTime Takes = 1 << iota
	TakeSession
	TakeProject
	TakeBranch
	TakeModel
	TakeText
	TakeSourceLine
)

// Form says when one of a parent's rows is shown.
type Form uint8

// The forms of a parent's row: shown whichever parent the group goes
// with, only when it goes with this parent (rows that replace the
// parent's own row), or only when it does not (the row they replace).
const (
	Always Form = iota
	WhenChosen
	WhenNotChosen
)

// RowText is a row's text and the shared texts it names, as a
// TextBuilder sets them (TextBuilder.Text).
type RowText struct {
	Text   string
	Shared *SharedRefs
}

// GroupRef is a row's place in a group: a member, or a row of one parent
// with its form and, for a row whose text depends on whether the group
// goes with its parent, the text in each case. A row copied from a grouped
// row shares its GroupRef, so the copy is a row of the same parent until
// it is registered again.
type GroupRef struct {
	group  *Group
	parent int
	takes  Takes
	form   Form
	// texts is the text when the group goes with the parent, then the
	// text when it does not; nil when the text is the same in both.
	texts *[2]RowText
}

// Parent registers a new parent of the group, with the attributes row
// holds now, and makes row one of its rows, shown in either case.
func (g *Group) Parent(row *Row) *Parent {
	parent := &Parent{group: g, index: len(g.parents)}
	g.parents = append(g.parents, *row)
	parent.Row(row, Always)
	return parent
}

// Row makes row one of the parent's rows, shown as form says.
func (p *Parent) Row(row *Row, form Form) {
	row.Group = &GroupRef{group: p.group, parent: p.index, form: form}
}

// Texts makes row one of the parent's rows, its text chosen when the
// group goes with the parent and notChosen otherwise; row holds the one
// of the output without a filter. A filter's pattern is matched against
// both texts of every such row, and against both again in the row of the
// parent the group goes with, so a text the group shows in one parent's
// row finds the group whichever of its parents holds it.
func (p *Parent) Texts(row *Row, chosen, notChosen RowText) {
	row.Group = &GroupRef{group: p.group, parent: p.index, texts: &[2]RowText{chosen, notChosen}}
	text := notChosen
	if p.index == 0 {
		text = chosen
	}
	row.Text, row.Shared = text.Text, text.Shared
}

// Member makes row a member of the group, taking what takes names from
// the parent the group goes with; row is given those attributes of the
// first parent now, so register it before the members.
func (g *Group) Member(row *Row, takes Takes) {
	row.Group = &GroupRef{group: g, parent: -1, takes: takes}
	if len(g.parents) > 0 {
		take(row, g.parents[0], takes)
	}
}

// take copies to row what takes names of parent.
func take(row *Row, parent Row, takes Takes) {
	for _, field := range []struct {
		take        Takes
		into, value *string
	}{{TakeTime, &row.TimestampUTC, &parent.TimestampUTC}, {TakeSession, &row.SessionID, &parent.SessionID},
		{TakeProject, &row.ProjectPath, &parent.ProjectPath}, {TakeBranch, &row.GitBranch, &parent.GitBranch},
		{TakeModel, &row.Model, &parent.Model}} {
		if takes&field.take != 0 {
			*field.into = *field.value
		}
	}
	if takes&TakeText != 0 {
		row.Text, row.Shared = parent.Text, parent.Shared
	}
	if takes&TakeSourceLine != 0 {
		row.SourceLine = parent.SourceLine
	}
}

// Grouped reports whether any row belongs to a group.
func Grouped(rows []Row) bool {
	for i := range rows {
		if rows[i].Group != nil {
			return true
		}
	}
	return false
}

// Choice is the parent each group goes with, by its index in the order
// the parser registered them; a group not in it goes with its first.
type Choice map[*Group]int

// Choose returns the parent each group goes with: the first, in the
// parser's order, with a row among kept, the rows a filter keeps of the
// rows the parsers gave. Without a filter there is no choice to make: a
// nil Choice gives every group its first parent.
func Choose(kept []Row) Choice {
	choice, _ := choose(kept)
	return choice
}

// choose is Choose, also returning its work: the rows it looked at.
func choose(kept []Row) (Choice, int) {
	choice, work := Choice{}, 0
	for i := range kept {
		work++
		ref := kept[i].Group
		if ref == nil || ref.parent < 0 {
			continue
		}
		if chosen, ok := choice[ref.group]; !ok || ref.parent < chosen {
			choice[ref.group] = ref.parent
		}
	}
	return choice, work
}

// Attach gives each grouped row its place once choice is settled: a
// member takes its attributes from the parent its group goes with, a row
// of a form not shown is left out, and a row with two texts holds the one
// for its case. It returns the rows kept, in order, in rows' own array;
// the times members take can change the order, so sort after it.
func Attach(rows []Row, choice Choice) []Row {
	kept, _ := attach(rows, choice)
	return kept
}

// attach is Attach, also returning its work: the rows it looked at and
// the grouped rows it placed.
func attach(rows []Row, choice Choice) ([]Row, int) {
	kept, work := rows[:0], 0
	for _, row := range rows {
		work++
		if row.Group == nil || len(row.Group.group.parents) == 0 {
			kept = append(kept, row)
			continue
		}
		work++
		if row.Group.place(&row, choice) {
			kept = append(kept, row)
		}
	}
	return kept, work
}

// place settles one row's place, reporting whether it is shown. Only a
// row of the chosen parent with two texts keeps its reference, for the
// filter to match both.
func (ref *GroupRef) place(row *Row, choice Choice) bool {
	chosen := choice[ref.group]
	if ref.parent < 0 {
		take(row, ref.group.parents[chosen], ref.takes)
		row.Group = nil
		return true
	}
	isChosen := ref.parent == chosen
	if ref.form == WhenChosen && !isChosen || ref.form == WhenNotChosen && isChosen {
		return false
	}
	if ref.texts != nil {
		text := ref.texts[1]
		if isChosen {
			text = ref.texts[0]
		}
		row.Text, row.Shared = text.Text, text.Shared
		if isChosen {
			return true
		}
	}
	row.Group = nil
	return true
}
