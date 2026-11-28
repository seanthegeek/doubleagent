package vscode

import (
	"math"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// node is a JSON value of a session being rebuilt from its mutation log:
// its arrays and objects change in place. A value that is neither is held
// as it was decoded. line is the log line of the record that made it.
type node struct {
	scalar   jsonvalue.Value
	line     int
	isArray  bool
	isObject bool
	items    itemList
	// keys holds an object's keys in the order they were added, a deleted
	// key left in place; slots gives each present key its position in
	// keys, so a delete need not search the list.
	keys    []string
	slots   map[string]int
	members map[string]*node
}

// add gives an object's key the value, a key it does not hold going last.
func (n *node) add(key string, value *node) {
	if _, ok := n.members[key]; !ok {
		n.slots[key] = len(n.keys)
		n.keys = append(n.keys, key)
	}
	n.members[key] = value
}

// newNode returns a decoded value as a node made by the record at log line
// line, its arrays and objects copied, and its height: 0 for a value that
// is neither, and one more than its highest item or member for an array or
// object, so a value put at a path of n keys reaches level n plus its
// height, the state itself being level 1.
func newNode(value jsonvalue.Value, line int) (*node, int) {
	switch {
	case value.IsArray():
		created, height := &node{isArray: true, line: line}, 0
		for _, item := range value.Items() {
			made, below := newNode(item, line)
			created.items.push(made)
			height = max(height, below)
		}
		return created, height + 1
	case value.IsObject():
		created, height := &node{isObject: true, line: line, slots: map[string]int{}, members: map[string]*node{}}, 0
		for _, member := range value.Members() {
			made, below := newNode(member.Value, line)
			created.add(member.Key, made)
			height = max(height, below)
		}
		return created, height + 1
	}
	return &node{scalar: value, line: line}, 0
}

// value returns the node as a decoded value.
func (n *node) value() jsonvalue.Value {
	switch {
	case n == nil:
		return jsonvalue.NewNull()
	case n.isArray:
		held := n.items.items()
		items := make([]jsonvalue.Value, len(held))
		for i, item := range held {
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

// isNull reports whether the node is absent or JSON null.
func (n *node) isNull() bool {
	return n == nil || (!n.isArray && !n.isObject && n.scalar.IsNull())
}

// mutationError is the reason a log record cannot be applied.
type mutationError string

// Error returns the reason.
func (e mutationError) Error() string { return string(e) }

// errTooDeep refuses a set or push whose path length plus the height of
// what it writes passes a decoded document's depth, so no recursion over
// the state needs more stack than one over a decoded document.
var errTooDeep = mutationError("the record nests the state deeper than " + strconv.Itoa(jsonvalue.MaxDepth) + " levels")

// integer returns a value written as an integer; one beyond int64 is held
// at the nearest bound, which is out of every range it is compared with.
func integer(value jsonvalue.Value) (int64, bool) {
	if !value.IsInt() {
		return 0, false
	}
	if number, ok := value.Int(); ok {
		return number, true
	}
	if value.Number()[0] == '-' {
		return math.MinInt64, true
	}
	return math.MaxInt64, true
}

// isKind reports whether a record's kind is the number n, compared by
// value, so 2 and 2.0 are the same kind.
func isKind(kind jsonvalue.Value, n int64) bool {
	return jsonvalue.Equal(kind, jsonvalue.NewInt(n))
}

// listIndex returns the position an integer key names in an array of size
// items, a negative key counting from the end, or false when it is out of
// range or not an integer.
func listIndex(key jsonvalue.Value, size int) (int, bool) {
	index, ok := integer(key)
	if !ok {
		return 0, false
	}
	if index < 0 {
		index += int64(size)
	}
	if index < 0 || index >= int64(size) {
		return 0, false
	}
	return int(index), true
}

// child returns parent[key]: an object's member by a string key, an
// array's item by an integer key, or false for a key that names nothing.
// Any other value is a leaf, which has no children.
func child(parent *node, key jsonvalue.Value) (*node, bool) {
	switch {
	case parent == nil:
		return nil, false
	case parent.isObject:
		found, ok := parent.members[key.StringValue()]
		return found, ok && key.IsString()
	case parent.isArray:
		if i, ok := listIndex(key, parent.items.Len()); ok {
			return parent.items.item(i), true
		}
	}
	return nil, false
}

// walk returns the node at path under state, or the error "path <path>
// not found".
func walk(state *node, path []jsonvalue.Value) (*node, error) {
	current := state
	for _, key := range path {
		next, ok := child(current, key)
		if !ok {
			return nil, mutationError("path " + jsonvalue.NewArray(path...).JSON() + " not found")
		}
		current = next
	}
	return current, nil
}

// set gives parent[key] the value: an array takes an integer key from 0
// to its length, its length appending; an object takes a string key.
func set(parent *node, key jsonvalue.Value, value *node) error {
	index, isInteger := integer(key)
	switch {
	case parent != nil && parent.isArray && isInteger:
		switch {
		case index == int64(parent.items.Len()):
			parent.items.push(value)
		case index >= 0 && index < int64(parent.items.Len()):
			parent.items.replace(int(index), value)
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

// remove deletes parent[key] when it is there; a key that names nothing,
// or a parent that cannot lose one, changes nothing. Neither an object's
// keys nor an array's items after the one deleted move; the entry deleted
// is counted in work.
func remove(parent *node, key jsonvalue.Value, work parserkit.Work) {
	switch {
	case parent == nil:
	case parent.isObject && key.IsString():
		if _, ok := parent.members[key.StringValue()]; ok {
			delete(parent.members, key.StringValue())
			delete(parent.slots, key.StringValue())
			work.Add("entries changed", 1)
		}
	case parent.isArray:
		if i, ok := listIndex(key, parent.items.Len()); ok {
			parent.items.remove(i)
			work.Add("entries changed", 1)
		}
	}
}

// addedNodes returns the nodes a record of kind 1 or 2 adds and the height
// it adds them at below its path: kind 1's value and its height, or kind
// 2's items (v's items when it is an array, else v) and the height of an
// array holding them. Any other kind adds nothing.
func addedNodes(kind, value jsonvalue.Value, line int) ([]*node, int) {
	switch {
	case isKind(kind, 1):
		created, height := newNode(value, line)
		return []*node{created}, height
	case isKind(kind, 2):
		pushed := []jsonvalue.Value{value}
		if value.IsArray() {
			pushed = value.Items()
		}
		nodes, height := make([]*node, len(pushed)), 0
		for i, item := range pushed {
			var below int
			nodes[i], below = newNode(item, line)
			height = max(height, below)
		}
		return nodes, height + 1
	}
	return nil, 0
}

// keyedMutation applies a record of kind 1, 2 or 3 at a non-empty path: a
// set of the one added node or a delete is done and gives nil, and a push
// gives the array to push to, made empty at the path when the path holds
// nothing.
func keyedMutation(state *node, kind jsonvalue.Value, path []jsonvalue.Value, added []*node,
	line int, work parserkit.Work) (*node, error) {
	parent, err := walk(state, path[:len(path)-1])
	if err != nil {
		return nil, err
	}
	key := path[len(path)-1]
	switch {
	case isKind(kind, 1):
		return nil, set(parent, key, added[0])
	case isKind(kind, 3):
		remove(parent, key, work)
		return nil, nil
	case !isKind(kind, 2):
		return nil, mutationError("unknown kind " + kind.JSON())
	}
	target, ok := child(parent, key)
	if !ok || target.isNull() {
		target = &node{isArray: true, line: line}
		if err := set(parent, key, target); err != nil {
			return nil, err
		}
	}
	return target, nil
}

// sliceStart returns how many items of a list of size items a truncation
// to start keeps: a negative start counts from the end, and a start beyond
// either end keeps nothing or everything.
func sliceStart(start int64, size int) int {
	if start < 0 {
		start = max(start+int64(size), 0)
	}
	return int(min(start, int64(size)))
}

// applyMutation applies one mutation-log record, at log line line, to
// state and returns the new state. Kind 0 replaces the state with v; kind 1
// sets v at path k; kind 2 truncates the array at k to i items, when i is
// given, and pushes v (each item of an array); kind 3 deletes the value at
// k. A set or push that would nest the state past jsonvalue.MaxDepth is
// refused (errTooDeep). The items pushed and deleted are counted in work.
func applyMutation(state *node, record jsonvalue.Value, line int, work parserkit.Work) (*node, error) {
	kind := record.Get("kind")
	if isKind(kind, 0) {
		created, _ := newNode(record.Get("v"), line)
		return created, nil
	}
	pathValue := record.Get("k")
	path := pathValue.Items()
	if !pathValue.IsArray() || (len(path) == 0 && !isKind(kind, 2)) {
		return state, mutationError("bad path " + pathValue.JSON())
	}
	added, height := addedNodes(kind, record.Get("v"), line)
	if (isKind(kind, 1) || isKind(kind, 2)) && len(path)+height > jsonvalue.MaxDepth {
		return state, errTooDeep
	}
	target := state
	if len(path) > 0 {
		var err error
		if target, err = keyedMutation(state, kind, path, added, line, work); target == nil || err != nil {
			return state, err
		}
	}
	if target == nil || !target.isArray {
		return state, mutationError("push to non-list at " + pathValue.JSON())
	}
	if start, ok := integer(record.Get("i")); ok {
		target.items.truncate(sliceStart(start, target.items.Len()))
	}
	for _, item := range added {
		target.items.push(item)
	}
	work.Add("entries changed", len(added))
	return state, nil
}
