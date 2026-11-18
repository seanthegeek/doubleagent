package vscode

import (
	"math"
	"slices"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// node is a JSON value of a session being rebuilt from its mutation log:
// its arrays and objects change in place. A value that is neither is held
// as it was decoded.
type node struct {
	scalar   jsonvalue.Value
	isArray  bool
	isObject bool
	items    []*node
	keys     []string
	members  map[string]*node
}

// newNode returns a decoded value as a node, its arrays and objects copied.
func newNode(value jsonvalue.Value) *node {
	switch {
	case value.IsArray():
		created := &node{isArray: true, items: make([]*node, 0, value.Len())}
		for _, item := range value.Items() {
			created.items = append(created.items, newNode(item))
		}
		return created
	case value.IsObject():
		created := &node{isObject: true, members: map[string]*node{}}
		for _, member := range value.Members() {
			created.keys = append(created.keys, member.Key)
			created.members[member.Key] = newNode(member.Value)
		}
		return created
	}
	return &node{scalar: value}
}

// value returns the node as a decoded value.
func (n *node) value() jsonvalue.Value {
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
		members := make([]jsonvalue.Member, len(n.keys))
		for i, key := range n.keys {
			members[i] = jsonvalue.Member{Key: key, Value: n.members[key].value()}
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
		if i, ok := listIndex(key, len(parent.items)); ok {
			return parent.items[i], true
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
		case index == int64(len(parent.items)):
			parent.items = append(parent.items, value)
		case index >= 0 && index < int64(len(parent.items)):
			parent.items[index] = value
		default:
			return mutationError("index " + key.JSON() + " out of range")
		}
		return nil
	case parent != nil && parent.isObject && key.IsString():
		if _, ok := parent.members[key.StringValue()]; !ok {
			parent.keys = append(parent.keys, key.StringValue())
		}
		parent.members[key.StringValue()] = value
		return nil
	}
	return mutationError("cannot set " + key.JSON())
}

// remove deletes parent[key] when it is there; a key that names nothing,
// or a parent that cannot lose one, changes nothing.
func remove(parent *node, key jsonvalue.Value) {
	switch {
	case parent == nil:
	case parent.isObject && key.IsString():
		if _, ok := parent.members[key.StringValue()]; ok {
			delete(parent.members, key.StringValue())
			parent.keys = slices.DeleteFunc(parent.keys, func(each string) bool { return each == key.StringValue() })
		}
	case parent.isArray:
		if i, ok := listIndex(key, len(parent.items)); ok {
			parent.items = slices.Delete(parent.items, i, i+1)
		}
	}
}

// applyMutation applies one mutation-log record to state and returns the
// new state. Kind 0 replaces the state with v; kind 1 sets v at path k; kind
// 2 truncates the array at k to i items, when i is given, and pushes v
// (each item of an array); kind 3 deletes the value at k.
func applyMutation(state *node, record jsonvalue.Value) (*node, error) {
	kind := record.Get("kind")
	if isKind(kind, 0) {
		return newNode(record.Get("v")), nil
	}
	pathValue := record.Get("k")
	path := pathValue.Items()
	if !pathValue.IsArray() || (len(path) == 0 && !isKind(kind, 2)) {
		return state, mutationError("bad path " + pathValue.JSON())
	}
	target := state
	if len(path) > 0 {
		var err error
		if target, err = keyedMutation(state, kind, path, record.Get("v")); target == nil || err != nil {
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
			target.items = append(target.items, newNode(item))
		}
	} else {
		target.items = append(target.items, newNode(items))
	}
	return state, nil
}

// keyedMutation applies a record of kind 1, 2 or 3 at a non-empty path: a
// set of value or a delete is done and gives nil, and a push gives the array to push
// to, made empty at the path when the path holds nothing.
func keyedMutation(state *node, kind jsonvalue.Value, path []jsonvalue.Value, value jsonvalue.Value) (*node, error) {
	parent, err := walk(state, path[:len(path)-1])
	if err != nil {
		return nil, err
	}
	key := path[len(path)-1]
	switch {
	case isKind(kind, 1):
		return nil, set(parent, key, newNode(value))
	case isKind(kind, 3):
		remove(parent, key)
		return nil, nil
	case !isKind(kind, 2):
		return nil, mutationError("unknown kind " + kind.JSON())
	}
	target, ok := child(parent, key)
	if !ok || target.isNull() {
		target = &node{isArray: true}
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
