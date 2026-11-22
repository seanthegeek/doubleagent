package vscode

import (
	"slices"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// The code below is the mutation log's replay as it was before array
// deletes stopped moving the items after them: each delete shifts the
// later items and their tracked lines down one place. It is kept as the
// reference the current replay is compared with over generated logs.

// shiftingNode is a JSON value of a session being rebuilt from its mutation log:
// its arrays and objects change in place. A value that is neither is held
// as it was decoded.
type shiftingNode struct {
	scalar   jsonvalue.Value
	isArray  bool
	isObject bool
	items    []*shiftingNode
	// keys holds an object's keys in the order they were added, a deleted
	// key left in place; slots gives each present key its position in
	// keys, so a delete need not search the list.
	keys    []string
	slots   map[string]int
	members map[string]*shiftingNode
}

// newShiftingNode returns a decoded value as a node, its arrays and objects copied.
func newShiftingNode(value jsonvalue.Value) *shiftingNode {
	switch {
	case value.IsArray():
		created := &shiftingNode{isArray: true, items: make([]*shiftingNode, 0, value.Len())}
		for _, item := range value.Items() {
			created.items = append(created.items, newShiftingNode(item))
		}
		return created
	case value.IsObject():
		created := &shiftingNode{isObject: true, slots: map[string]int{}, members: map[string]*shiftingNode{}}
		for _, member := range value.Members() {
			created.add(member.Key, newShiftingNode(member.Value))
		}
		return created
	}
	return &shiftingNode{scalar: value}
}

// value returns the node as a decoded value.
func (n *shiftingNode) value() jsonvalue.Value {
	switch {
	case n == nil:
		return jsonvalue.NewNull()
	case n.isArray:
		items := make([]jsonvalue.Value, len(n.items))
		for i, item := range n.items {
			items[i] = item.value()
		}
		return jsonvalue.NewArray(items...)
	case n.isObject:
		members := make([]jsonvalue.Member, 0, len(n.members))
		for i, key := range n.keys {
			if slot, ok := n.slots[key]; ok && slot == i {
				members = append(members, jsonvalue.Member{Key: key, Value: n.members[key].value()})
			}
		}
		return jsonvalue.NewObject(members...)
	}
	return n.scalar
}

// add gives an object's key the value, a key it does not hold going last.
func (n *shiftingNode) add(key string, value *shiftingNode) {
	if _, ok := n.members[key]; !ok {
		n.slots[key] = len(n.keys)
		n.keys = append(n.keys, key)
	}
	n.members[key] = value
}

// isNull reports whether the node is absent or JSON null.
func (n *shiftingNode) isNull() bool {
	return n == nil || (!n.isArray && !n.isObject && n.scalar.IsNull())
}

// shiftingChild returns parent[key]: an object's member by a string key, an
// array's item by an integer key, or false for a key that names nothing.
// Any other value is a leaf, which has no children.
func shiftingChild(parent *shiftingNode, key jsonvalue.Value) (*shiftingNode, bool) {
	switch {
	case parent == nil:
		return nil, false
	case parent.isObject:
		found, ok := parent.members[key.StringValue()]
		return found, ok && key.IsString()
	case parent.isArray:
		if i, ok := listIndex(key, len(parent.items)); ok {
			return parent.items[i], true
		}
	}
	return nil, false
}

// shiftingWalk returns the node at path under state, or the error "path <path>
// not found".
func shiftingWalk(state *shiftingNode, path []jsonvalue.Value) (*shiftingNode, error) {
	current := state
	for _, key := range path {
		next, ok := shiftingChild(current, key)
		if !ok {
			return nil, mutationError("path " + jsonvalue.NewArray(path...).JSON() + " not found")
		}
		current = next
	}
	return current, nil
}

// shiftingSet gives parent[key] the value: an array takes an integer key from 0
// to its length, its length appending; an object takes a string key.
func shiftingSet(parent *shiftingNode, key jsonvalue.Value, value *shiftingNode) error {
	index, isInteger := integer(key)
	switch {
	case parent != nil && parent.isArray && isInteger:
		switch {
		case index == int64(len(parent.items)):
			parent.items = append(parent.items, value)
		case index >= 0 && index < int64(len(parent.items)):
			parent.items[index] = value
		default:
			return mutationError("index " + key.JSON() + " out of range")
		}
		return nil
	case parent != nil && parent.isObject && key.IsString():
		parent.add(key.StringValue(), value)
		return nil
	}
	return mutationError("cannot set " + key.JSON())
}

// shiftingRemove deletes parent[key] when it is there; a key that names nothing,
// or a parent that cannot lose one, changes nothing. It counts in work the
// keys and items it moves or looks at.
func shiftingRemove(parent *shiftingNode, key jsonvalue.Value, work parser.Work) {
	switch {
	case parent == nil:
	case parent.isObject && key.IsString():
		if _, ok := parent.members[key.StringValue()]; ok {
			delete(parent.members, key.StringValue())
			delete(parent.slots, key.StringValue())
			work.Add("entries moved", 1)
		}
	case parent.isArray:
		if i, ok := listIndex(key, len(parent.items)); ok {
			parent.items = slices.Delete(parent.items, i, i+1)
			work.Add("entries moved", len(parent.items)-i)
		}
	}
}

// shiftingApply applies one mutation-log record to state and returns the
// new state. Kind 0 replaces the state with v; kind 1 sets v at path k; kind
// 2 truncates the array at k to i items, when i is given, and pushes v
// (each item of an array); kind 3 deletes the value at k, its work counted
// in work.
func shiftingApply(state *shiftingNode, record jsonvalue.Value, work parser.Work) (*shiftingNode, error) {
	kind := record.Get("kind")
	if isKind(kind, 0) {
		return newShiftingNode(record.Get("v")), nil
	}
	pathValue := record.Get("k")
	path := pathValue.Items()
	if !pathValue.IsArray() || (len(path) == 0 && !isKind(kind, 2)) {
		return state, mutationError("bad path " + pathValue.JSON())
	}
	target := state
	if len(path) > 0 {
		var err error
		if target, err = shiftingKeyed(state, kind, path, record.Get("v"), work); target == nil || err != nil {
			return state, err
		}
	}
	if target == nil || !target.isArray {
		return state, mutationError("push to non-list at " + pathValue.JSON())
	}
	if start, ok := integer(record.Get("i")); ok {
		target.items = target.items[:sliceStart(start, len(target.items))]
	}
	if items := record.Get("v"); items.IsArray() {
		for _, item := range items.Items() {
			target.items = append(target.items, newShiftingNode(item))
		}
	} else {
		target.items = append(target.items, newShiftingNode(items))
	}
	return state, nil
}

// shiftingKeyed applies a record of kind 1, 2 or 3 at a non-empty path: a
// shiftingSet of value or a delete is done and gives nil, and a push gives the array to push
// to, made empty at the path when the path holds nothing.
func shiftingKeyed(state *shiftingNode, kind jsonvalue.Value, path []jsonvalue.Value, value jsonvalue.Value,
	work parser.Work) (*shiftingNode, error) {
	parent, err := shiftingWalk(state, path[:len(path)-1])
	if err != nil {
		return nil, err
	}
	key := path[len(path)-1]
	switch {
	case isKind(kind, 1):
		return nil, shiftingSet(parent, key, newShiftingNode(value))
	case isKind(kind, 3):
		shiftingRemove(parent, key, work)
		return nil, nil
	case !isKind(kind, 2):
		return nil, mutationError("unknown kind " + kind.JSON())
	}
	target, ok := shiftingChild(parent, key)
	if !ok || target.isNull() {
		target = &shiftingNode{isArray: true}
		if err := shiftingSet(parent, key, target); err != nil {
			return nil, err
		}
	}
	return target, nil
}

// shiftingRequestCount returns how many requests the state holds.
func shiftingRequestCount(state *shiftingNode) int64 {
	if state == nil || !state.isObject {
		return 0
	}
	requests := state.members["requests"]
	if requests == nil || !requests.isArray {
		return 0
	}
	return int64(len(requests.items))
}

// shiftingTrack remembers the log line that created each request, given how many
// requests there were before and after the record was applied: every
// request of a replaced state or of a list set whole, the requests a push
// added at the end of the list (whatever the truncation before it), a
// request set by its index, and a deleted request's followers moved down
// one place. Before the record, lines holds a line for each of the before
// requests and no other, so only those from the first changed one on are
// looked at; the entries set or moved are counted in work.
func shiftingTrack(record jsonvalue.Value, before, after int64, number int, lines requestLines, work parser.Work) {
	kind := record.Get("kind")
	path := record.Get("k").Items()
	switch {
	case isKind(kind, 0):
		clear(lines)
		lines[sessionStart] = number
		for i := range after {
			lines[i] = number
		}
		work.Add("entries moved", int(after))
	case len(path) == 1 && isRequests(path[0]):
		start := int64(0)
		if isKind(kind, 2) {
			start = after - pushedCount(record.Get("v"))
		}
		for i := start; i < before; i++ {
			delete(lines, i)
		}
		for i := start; i < after; i++ {
			lines[i] = number
		}
		work.Add("entries moved", int(max(before, after)-start))
	case len(path) == 2 && isRequests(path[0]) && isKind(kind, 1) && path[1].IsInt():
		index, _ := integer(path[1])
		lines[index] = number
	case len(path) == 2 && isRequests(path[0]) && isKind(kind, 3):
		if removed, ok := listIndex(path[1], int(before)); ok {
			for i := int64(removed); i < after; i++ {
				if line, ok := lines[i+1]; ok {
					lines[i] = line
				} else {
					delete(lines, i)
				}
			}
			delete(lines, after)
			work.Add("entries moved", int(after)-removed)
		}
	}
}

// isRequests reports whether a path key is "requests".
func isRequests(key jsonvalue.Value) bool {
	return key.IsString() && key.StringValue() == "requests"
}

// pushedCount returns how many items a push of value adds: each item of
// an array, or the value itself as one item.
func pushedCount(value jsonvalue.Value) int64 {
	if value.IsArray() {
		return int64(value.Len())
	}
	return 1
}
