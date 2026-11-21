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
	"github.com/seanthegeek/doubleagent/internal/timeline"
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
	// work counts the entries ModelAt has looked at, which a test holds to
	// a binary search.
	work parser.Work
	// group is the group of the task's id when its index repeats it, and
	// startFromItem whether Start is the index entry's ts.
	group         *timeline.Group
	startFromItem bool
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
	context.Start, context.startFromItem = taskStart(taskID, item, metadata)
	return context
}

// Member makes row, a row of the task, a member of the group of the task's
// id when the index repeats it, taking from the entry the group goes with
// what it took from the first: the project, the time when ownTime is false
// and Start is the entry's ts, and the model when ownModel is false and no
// model_usage entry stands in for the entry's modelId.
func (c *TaskContext) Member(row *timeline.Row, ownTime, ownModel bool) {
	if c.group == nil {
		return
	}
	takes := timeline.TakeProject
	if !ownTime && c.startFromItem {
		takes |= timeline.TakeTime
	}
	if !ownModel && len(c.usage) == 0 {
		takes |= timeline.TakeModel
	}
	c.group.Member(row, takes)
}

// epochOf returns the time of a model_usage entry in epoch milliseconds:
// TimeOf's, else a string holding a number with spaces around it, as a
// hand-edited file may write it, past the float range infinite by its sign
// as TimeOf gives a number. Any other value, an empty one and NaN text are
// 0, so an entry without a readable time counts as the earliest.
func epochOf(value jsonvalue.Value) float64 {
	if moment, ok := TimeOf(value); ok {
		return moment
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(value.StringValue()), 64)
	if (err != nil && !errors.Is(err, strconv.ErrRange)) || math.IsNaN(number) {
		return 0
	}
	return number
}

// taskStart returns the start of a task: its id when that is a millisecond
// time, else the earliest numeric ts of the metadata's model_usage and
// environment_history entries, else the index item's ts, and whether it is
// the index item's.
func taskStart(taskID string, item, metadata jsonvalue.Value) (string, bool) {
	if millisecondID.MatchString(taskID) {
		epoch, _ := strconv.ParseInt(taskID, 10, 64)
		return timeutil.FromInt(epoch), false
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
		return timeutil.FromValue(earliest), false
	}
	return timeutil.FromValue(item.Get("ts")), true
}

// numberLess reports whether one number is below another.
func numberLess(left, right jsonvalue.Value) bool {
	return OrderOf(left) < OrderOf(right)
}

// ModelAt returns the model in use at an epoch-millisecond time (TimeOf):
// the last model_usage entry at or before it, the earliest entry for a time
// that is not a number, and the index item's model when there are no
// entries. The entries are sorted by time, so each lookup is a binary
// search.
func (c *TaskContext) ModelAt(timestamp jsonvalue.Value) string {
	if len(c.usage) == 0 {
		return c.defaultModel
	}
	moment, ok := TimeOf(timestamp)
	if !ok {
		return c.usage[0].model
	}
	after := sort.Search(len(c.usage), func(i int) bool {
		c.work.Add("entries compared", 1)
		return c.usage[i].since > moment
	})
	return c.usage[max(after-1, 0)].model
}
