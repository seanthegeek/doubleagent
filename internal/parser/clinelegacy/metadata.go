package clinelegacy

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// metadataEvent is one dated event of task_metadata.json: its time, its
// position in its list, the model of a model_usage entry and either its
// text or, for a model_usage entry, the entry, whose text depends on the
// entries before it.
type metadataEvent struct {
	timestamp pyjson.Value
	position  int
	model     string
	text      string
	usage     pyjson.Value
}

// sortKey returns the number an event is ordered by: its time when that is
// a number, else 0.
func (e metadataEvent) sortKey() float64 {
	key, _ := NumberOf(e.timestamp)
	return key
}

// parseMetadata emits a system row per dated event of task_metadata.json in
// time order: files read or edited, model and mode changes, environments.
func (p *TaskParser) parseMetadata(artifact *input.Artifact, context *TaskContext, emit parser.Emit) {
	var problems parser.Problems
	data, _ := parser.LoadJSON(artifact.DiskPath, &problems)
	if !data.IsObject() {
		if len(problems) == 0 {
			problems.Add(0, "task_metadata.json is not a JSON object")
		}
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, context.TaskID, context.ProjectPath))
		return
	}
	events := fileEvents(data.Get("files_in_context"))
	zero := pyjson.NewInt(0)
	for position, entry := range data.Get("model_usage").Items() {
		if entry.IsObject() {
			events = append(events, metadataEvent{timestamp: FirstTruthy(entry.Get("ts"), zero),
				position: position + 1, model: parser.Str(entry.Get("model_id")), usage: entry})
		}
	}
	for position, entry := range data.Get("environment_history").Items() {
		if entry.IsObject() {
			events = append(events, metadataEvent{timestamp: FirstTruthy(entry.Get("ts"), zero),
				position: position + 1, text: "environment: " + FormatCounts(entry, "os_name",
					"os_version", "os_arch", "host_name", "host_version", "cline_version")})
		}
	}
	slices.SortStableFunc(events, func(left, right metadataEvent) int {
		return CompareNumbers(left.sortKey(), right.sortKey())
	})
	models, modes := parser.Tracker{}, parser.Tracker{}
	for _, event := range events {
		for _, text := range eventTexts(event, context.TaskID, models, modes) {
			row := p.Row(artifact, context, event.position, timeutil.FromValue(event.timestamp), event.model)
			row.TurnType = timeline.TurnSystem
			row.Text = timeline.Compact(text)
			emit(row)
		}
	}
}

// eventTexts returns the row texts of one event: a model_usage entry gives
// a model: row when the model changes and a permission mode: row when the
// mode does, any other event its own text.
func eventTexts(event metadataEvent, taskID string, models, modes parser.Tracker) []string {
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
	if mode := modes.Change("permission mode", taskID, parser.Str(event.usage.Get("mode"))); mode != "" {
		texts = append(texts, mode)
	}
	return texts
}

// fileEvents returns an event per numeric *_date field of each
// files_in_context entry, in key order, once per path, field and time.
func fileEvents(files pyjson.Value) []metadataEvent {
	var events []metadataEvent
	seen := map[string]bool{}
	for position, entry := range files.Items() {
		if !entry.IsObject() {
			continue
		}
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
				text: "file " + strings.TrimSuffix(key, "_date") + ": " + parser.Str(entry.Get("path")) +
					" (source=" + parser.Str(entry.Get("record_source")) + " state=" +
					parser.Str(entry.Get("record_state")) + ")"})
		}
	}
	return events
}

// hashKey returns a value as a key that equal values share, as Python's set
// compares them: a number (or boolean) by its value, so 1, 1.0 and true are
// one key, and any other value by its kind and JSON text.
func hashKey(value pyjson.Value) string {
	if number, ok := NumberOf(value); ok {
		return "n" + pyjson.Compact(pyjson.NewFloat(number))
	}
	return "v" + pyjson.Sorted(value)
}
