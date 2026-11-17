package clinelegacy

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// millisecondID is a task id that is a Date.now() value: a millisecond
// epoch of 12 to 14 digits.
var millisecondID = regexp.MustCompile(`^[0-9]{12,14}\z`)

// modelUse is one model_usage entry: when the model came into use, in
// epoch milliseconds, and which.
type modelUse struct {
	since float64
	model string
}

// TaskContext is what one task directory knows about itself: its id,
// project, models and a start time for records that carry none.
type TaskContext struct {
	TaskID      string
	ProjectPath string
	// Start is the time of a record without its own.
	Start        string
	defaultModel string
	usage        []modelUse
}

// newTaskContext reads a task's project and models from its index item
// and metadata, either of which may be null or of another type.
func newTaskContext(taskID string, item, metadata jsonvalue.Value, projectKey string) *TaskContext {
	if !item.IsObject() {
		item = jsonvalue.NewNull()
	}
	if !metadata.IsObject() {
		metadata = jsonvalue.NewNull()
	}
	context := &TaskContext{TaskID: taskID, ProjectPath: parser.Str(item.Get(projectKey)),
		defaultModel: parser.Str(item.Get("modelId"))}
	for _, entry := range metadata.Get("model_usage").Items() {
		if !entry.IsObject() || entry.Get("model_id").IsEmpty() {
			continue
		}
		since, _ := pythonFloat(FirstTruthy(entry.Get("ts"), jsonvalue.NewInt(0)))
		context.usage = append(context.usage, modelUse{since: since, model: parser.Str(entry.Get("model_id"))})
	}
	slices.SortStableFunc(context.usage, func(left, right modelUse) int {
		if order := CompareNumbers(left.since, right.since); order != 0 {
			return order
		}
		return strings.Compare(left.model, right.model)
	})
	context.Start = taskStart(taskID, item, metadata)
	return context
}

// pythonFloat returns a value as Python's float() gives it: a number, a
// boolean as 0 or 1, and a string of a decimal number with surrounding
// whitespace. Anything else gives false.
func pythonFloat(value jsonvalue.Value) (float64, bool) {
	if !value.IsString() {
		return NumberOf(value)
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(value.StringValue()), 64)
	return number, err == nil
}

// taskStart returns the start of a task: its id when that is a millisecond
// time, else the earliest numeric ts of the metadata's model_usage and
// environment_history entries, else the index item's ts.
func taskStart(taskID string, item, metadata jsonvalue.Value) string {
	if millisecondID.MatchString(taskID) {
		epoch, _ := strconv.ParseInt(taskID, 10, 64)
		return timeutil.FromInt(epoch)
	}
	earliest, found := jsonvalue.NewNull(), false
	for _, key := range []string{"model_usage", "environment_history"} {
		for _, entry := range metadata.Get(key).Items() {
			stamp := entry.Get("ts")
			if !IsNumber(stamp) {
				continue
			}
			if !found || numberLess(stamp, earliest) {
				earliest, found = stamp, true
			}
		}
	}
	if found {
		return timeutil.FromValue(earliest)
	}
	return timeutil.FromValue(item.Get("ts"))
}

// numberLess reports whether one number is below another.
func numberLess(left, right jsonvalue.Value) bool {
	leftNumber, _ := NumberOf(left)
	rightNumber, _ := NumberOf(right)
	return leftNumber < rightNumber
}

// ModelAt returns the model in use at an epoch-millisecond time: the last
// model_usage entry at or before it, the earliest entry for a time that is
// not a number, and the index item's model when there are no entries.
func (c *TaskContext) ModelAt(timestamp jsonvalue.Value) string {
	if len(c.usage) == 0 {
		return c.defaultModel
	}
	chosen := c.usage[0].model
	if moment, ok := NumberOf(timestamp); ok {
		for _, use := range c.usage {
			if use.since <= moment {
				chosen = use.model
			}
		}
	}
	return chosen
}
