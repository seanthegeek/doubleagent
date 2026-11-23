package clinelegacy

import (
	"cmp"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// metadataEvent is one dated event of task_metadata.json: its time, its
// position in its list, the model of a model_usage entry and either its
// text or, for a model_usage entry, the entry, whose text depends on the
// entries before it.
type metadataEvent struct {
	timestamp jsonvalue.Value
	position  int
	model     string
	text      string
	usage     jsonvalue.Value
}

// sortKey returns the number an event is ordered by: its time as OrderOf
// orders it.
func (e metadataEvent) sortKey() float64 {
	return OrderOf(e.timestamp)
}

// parseMetadata emits a system row per dated event of task_metadata.json in
// time order: files read or edited, model and mode changes, environments. A
// files_in_context, model_usage or environment_history that is not an array
// adds a problem.
func (p *TaskParser) parseMetadata(artifact *input.Artifact, context *TaskContext, emit parserkit.Emit) {
	var problems parserkit.Problems
	data, _ := parserkit.LoadJSON(artifact, artifact.DiskPath, &problems)
	if !data.IsObject() {
		if len(problems) == 0 {
			problems.Add(0, "task_metadata.json is not a JSON object")
		}
		parserkit.EmitAll(emit, p.TaskErrorRows(artifact, problems, context))
		return
	}
	events := fileEvents(data.Get("files_in_context"), &problems)
	for position, entry := range parserkit.EntriesOfKind(data.Get("model_usage"), "model_usage", jsonvalue.Object, 0,
		&problems) {
		events = append(events, metadataEvent{timestamp: timeOrZero(entry),
			position: position + 1, model: parserkit.Text(entry.Get("model_id")), usage: entry})
	}
	for position, entry := range parserkit.EntriesOfKind(data.Get("environment_history"), "environment_history",
		jsonvalue.Object, 0, &problems) {
		events = append(events, metadataEvent{timestamp: timeOrZero(entry),
			position: position + 1, text: "environment: " + FormatCounts(entry, "os_name",
				"os_version", "os_arch", "host_name", "host_version", "cline_version")})
	}
	slices.SortStableFunc(events, func(left, right metadataEvent) int {
		return cmp.Compare(left.sortKey(), right.sortKey())
	})
	models, modes := parserkit.Tracker{}, parserkit.Tracker{}
	for _, event := range events {
		for _, text := range eventTexts(event, context.TaskID, models, modes) {
			row := p.Row(artifact, context, event.position, timeutil.FromValue(event.timestamp), event.model)
			context.Member(&row, true, true)
			row.TurnType = timeline.TurnSystem
			row.Text = timeline.Compact(text)
			emit(row)
		}
	}
	parserkit.EmitAll(emit, p.TaskErrorRows(artifact, problems, context))
}

// timeOrZero returns an entry's ts, or the number 0 when it is empty, so an
// entry without a time sorts first and shows the epoch.
func timeOrZero(entry jsonvalue.Value) jsonvalue.Value {
	if stamp := entry.Get("ts"); !stamp.IsEmpty() {
		return stamp
	}
	return jsonvalue.NewInt(0)
}

// eventTexts returns the row texts of one event: a model_usage entry gives
// a model: row when the model changes and a permission mode: row when the
// mode does, any other event its own text.
func eventTexts(event metadataEvent, taskID string, models, modes parserkit.Tracker) []string {
	if event.usage.IsNull() {
		return []string{event.text}
	}
	var texts []string
	if change := models.Change("model", taskID, event.model); change != "" {
		if extra := FormatCounts(event.usage, "model_provider_id", "mode"); extra != "" {
			change += " " + extra
		}
		texts = append(texts, change)
	}
	if mode := modes.Change("permission mode", taskID, parserkit.Text(event.usage.Get("mode"))); mode != "" {
		texts = append(texts, mode)
	}
	return texts
}

// fileEvents returns an event per numeric *_date field of each
// files_in_context entry, in key order, once per path, field and time. A
// files_in_context that is not an array adds a problem.
func fileEvents(files jsonvalue.Value, problems *parserkit.Problems) []metadataEvent {
	var events []metadataEvent
	seen := map[string]bool{}
	for position, entry := range parserkit.EntriesOfKind(files, "files_in_context", jsonvalue.Object, 0, problems) {
		keys := slices.Clone(entry.Keys())
		slices.Sort(keys)
		for _, key := range keys {
			timestamp := entry.Get(key)
			if !strings.HasSuffix(key, "_date") || !IsNumber(timestamp) {
				continue
			}
			signature := hashKey(entry.Get("path")) + "\x00" + key + "\x00" + hashKey(timestamp)
			if seen[signature] {
				continue
			}
			seen[signature] = true
			events = append(events, metadataEvent{timestamp: timestamp, position: position + 1,
				text: "file " + strings.TrimSuffix(key, "_date") + ": " + parserkit.Text(entry.Get("path")) +
					" (source=" + parserkit.Text(entry.Get("record_source")) + " state=" +
					parserkit.Text(entry.Get("record_state")) + ")"})
		}
	}
	return events
}

// hashKey returns a value as a key that equal values share: a number (or
// boolean) by its value, so 1, 1.0 and true are one key (numbers are
// compared by value), and any other value by its kind and JSON text.
func hashKey(value jsonvalue.Value) string {
	if number, ok := NumberOf(value); ok {
		return "n" + jsonvalue.NewFloat(number).JSON()
	}
	return "v" + value.JSON()
}
