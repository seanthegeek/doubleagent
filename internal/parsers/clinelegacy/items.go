package clinelegacy

import (
	"path"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// itemFile is a task's own index entry in the layout Roo Code and Kilo
// Code share; tasks/_index.json holds every task's.
const itemFile = "history_item.json"

// HistoryItem returns the task's history_item.json, else its
// tasks/_index.json entry, with the group of its id when the index repeats
// it, else null: the index of Roo Code's layout, a TaskParser's Family when
// it is given none.
func (p *TaskParser) HistoryItem(task Task) (jsonvalue.Value, *timeline.Group) {
	var ignored parserkit.Problems
	// The item is also parsed as its own artifact, whose parse reports its
	// repeated member names; a lookup from a task's parse notes none.
	if item, _ := parserkit.LoadJSON(nil, task.InDir(itemFile), &ignored); item.IsObject() {
		return item, nil
	}
	indexPath, found := task.InRoot("tasks/_index.json")
	if !found {
		return jsonvalue.NewNull(), nil
	}
	return p.CachedEntry(indexPath, "entries", task.TaskID, nil), p.EntryGroup(indexPath, "entries", task.TaskID, nil)
}

// IndexKinds returns the index files of Roo Code's layout under the root:
// each task's history_item.json and tasks/_index.json.
func (p *TaskParser) IndexKinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.PatternKind("history_item.json", p.itemPattern),
		parserkit.PatternKind("tasks _index.json", p.indexPattern)}
}

// IndexMatch reports whether the artifact is one of IndexKinds' files.
func (p *TaskParser) IndexMatch(artifact *input.Artifact) bool {
	return p.itemPattern.MatchString(artifact.Rel) || p.indexPattern.MatchString(artifact.Rel)
}

// historyRows emits the task row of an index entry, then "permission
// mode: <mode>" from its mode, the slug that decides the tool groups the
// task may use; both are the rows of a new parent of group when it is set.
func (p *TaskParser) historyRows(artifact *input.Artifact, item jsonvalue.Value, position int,
	group *timeline.Group, emit parserkit.Emit) {
	row := p.HistoryRow(artifact, item, position)
	if group != nil {
		group.Parent(&row)
	}
	emit(row)
	if mode := item.Get("mode"); mode.IsString() && mode.StringValue() != "" {
		row.Text = timeline.Compact("permission mode: " + mode.StringValue())
		emit(row)
	}
}

// parseIndex emits the task rows of the tasks/_index.json entries whose
// history_item.json is gone; an id that is not a plain file name has none.
// The rows of each entry of an id the index repeats are a parent of the
// id's group, which the task's files share (EntryGroup).
func (p *TaskParser) parseIndex(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	index, _ := parserkit.LoadJSON(artifact, artifact.DiskPath, &problems)
	if !index.Get("entries").IsArray() {
		if len(problems) == 0 {
			problems.Add(0, "_index.json has no entries array")
		}
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	for i, entry := range parserkit.EntriesOfKind(index.Get("entries"), "entries", jsonvalue.Object, 0, &problems) {
		entryID := entry.Get("id").Text()
		taskID, named := parserkit.Component(entryID)
		if item, ok := parserkit.Beside(artifact, taskID, itemFile); named && ok && parserkit.RegularFile(item) {
			continue
		}
		var group *timeline.Group
		if indexPath, shared := parserkit.Beside(artifact, "_index.json"); shared {
			group = p.EntryGroup(indexPath, "entries", entryID, nil)
		}
		p.historyRows(artifact, entry, i+1, group, emit)
	}
}

// ParseIndexFile emits the rows of one of IndexKinds' files: the task row
// of a history_item.json, or those of tasks/_index.json.
func (p *TaskParser) ParseIndexFile(artifact *input.Artifact, emit parserkit.Emit) {
	if p.indexPattern.MatchString(artifact.Rel) {
		p.parseIndex(artifact, emit)
		return
	}
	var problems parserkit.Problems
	item, _ := parserkit.LoadJSON(artifact, artifact.DiskPath, &problems)
	if item.IsObject() {
		p.historyRows(artifact, item, 1, nil, emit)
		return
	}
	if len(problems) == 0 {
		problems.Add(0, "history_item.json is not a JSON object")
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, path.Base(path.Dir(artifact.Rel)), ""))
}
