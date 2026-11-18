package clinelegacy

import (
	"cmp"
	"errors"
	"math"
	"regexp"
	"slices"
	"sort"
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
	// compared counts the entries ModelAt has looked at, the work a test
	// holds to a binary search.
	compared int
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
	context := &TaskContext{TaskID: taskID, ProjectPath: parser.Text(item.Get(projectKey)),
		defaultModel: parser.Text(item.Get("modelId"))}
	for _, entry := range metadata.Get("model_usage").Items() {
		if !entry.IsObject() || entry.Get("model_id").IsEmpty() {
			continue
		}
		context.usage = append(context.usage, modelUse{since: epochOf(entry.Get("ts")),
			model: parser.Text(entry.Get("model_id"))})
	}
	slices.SortStableFunc(context.usage, func(left, right modelUse) int {
		return cmp.Or(cmp.Compare(left.since, right.since), strings.Compare(left.model, right.model))
	})
	context.Start = taskStart(taskID, item, metadata)
	return context
}

// epochOf returns the time of a model_usage entry in epoch milliseconds:
// a number (NumberOf), or a string holding one with spaces around it, as a
// hand-edited file may write it. Any other value, an empty one and a NaN
// are 0, so an entry without a readable time counts as the earliest. A
// number past the float range is infinite, as strconv.ParseFloat gives it.
func epochOf(value jsonvalue.Value) float64 {
	number, ok := NumberOf(value)
	if value.IsString() {
		var err error
		number, err = strconv.ParseFloat(strings.TrimSpace(value.StringValue()), 64)
		ok = err == nil || errors.Is(err, strconv.ErrRange)
	}
	if !ok || math.IsNaN(number) {
		return 0
	}
	return number
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
	return OrderOf(left) < OrderOf(right)
}

// ModelAt returns the model in use at an epoch-millisecond time: the last
// model_usage entry at or before it, the earliest entry for a time that is
// not a number, and the index item's model when there are no entries. The
// entries are sorted by time, so each lookup is a binary search.
func (c *TaskContext) ModelAt(timestamp jsonvalue.Value) string {
	if len(c.usage) == 0 {
		return c.defaultModel
	}
	moment, ok := NumberOf(timestamp)
	if !ok {
		return c.usage[0].model
	}
	after := sort.Search(len(c.usage), func(i int) bool {
		c.compared++
		return c.usage[i].since > moment
	})
	return c.usage[max(after-1, 0)].model
}
