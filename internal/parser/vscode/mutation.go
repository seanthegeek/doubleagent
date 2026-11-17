package vscode

import (
	"math"
	"slices"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// node is a JSON value of a session being rebuilt from its mutation log:
// its arrays and objects change in place. A value that is neither is held
// as it was decoded.
type node struct {
	scalar   pyjson.Value
	isArray  bool
	isObject bool
	items    []*node
	keys     []string
	members  map[string]*node
}

// newNode returns a decoded value as a node, its arrays and objects copied.
func newNode(value pyjson.Value) *node {
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
func (n *node) value() pyjson.Value {
	switch {
	case n == nil:
		return pyjson.NewNull()
	case n.isArray:
		items := make([]pyjson.Value, len(n.items))
		for i, item := range n.items {
			items[i] = item.value()
		}
		return pyjson.NewArray(items...)
	case n.isObject:
		members := make([]pyjson.Member, len(n.keys))
		for i, key := range n.keys {
			members[i] = pyjson.Member{Key: key, Value: n.members[key].value()}
		}
		return pyjson.NewObject(members...)
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

// pyInt returns a value as Python's isinstance(value, int) reads it: an
// integer, or a boolean as 0 or 1; an integer beyond int64 is held at the
// nearest bound, which is out of every range it is compared with.
func pyInt(value pyjson.Value) (int64, bool) {
	if flag, ok := value.Bool(); ok {
		if flag {
			return 1, true
		}
		return 0, true
	}
	if !value.IsInt() {
		return 0, false
	}
	if number, ok := value.Int(); ok {
		return number, true
	}
	if value.NumberText()[0] == '-' {
		return math.MinInt64, true
	}
	return math.MaxInt64, true
}

// equalsInt reports whether value == n in Python: a number of that value,
// in integer or float form, or a boolean equal to it.
func equalsInt(value pyjson.Value, n int64) bool {
	if number, ok := pyInt(value); ok {
		return number == n
	}
	number, ok := value.Float()
	return ok && number == float64(n)
}

// listIndex returns the position a Python index names in a list of size
// items, a negative index counting from the end, or false when it is out of
// range or not an integer.
func listIndex(key pyjson.Value, size int) (int, bool) {
	index, ok := pyInt(key)
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

// child returns parent[key] as Python reads it, or false where Python
// raises KeyError, IndexError or TypeError: an object by a string key, an
// array or a string by an integer index.
func child(parent *node, key pyjson.Value) (*node, bool) {
	switch {
	case parent == nil:
		return nil, false
	case parent.isObject:
		found, ok := parent.members[key.Text()]
		return found, ok && key.IsString()
	case parent.isArray:
		if i, ok := listIndex(key, len(parent.items)); ok {
			return parent.items[i], true
		}
	case parent.scalar.IsString():
		characters := parser.Elements(parent.scalar)
		if i, ok := listIndex(key, len(characters)); ok {
			return &node{scalar: characters[i]}, true
		}
	}
	return nil, false
}

// walk returns the node at path under state, or the error "path <path>
// not found".
func walk(state *node, path []pyjson.Value) (*node, error) {
	current := state
	for _, key := range path {
		next, ok := child(current, key)
		if !ok {
			return nil, mutationError("path " + pyjson.NewArray(path...).Repr() + " not found")
		}
		current = next
	}
	return current, nil
}

// set gives parent[key] the value as Python's assignment does: an array
// takes an index up to its length, one past the end appending; an object
// takes a string key.
func set(parent *node, key pyjson.Value, value *node) error {
	switch {
	case parent != nil && parent.isArray && isIntLike(key):
		index, _ := pyInt(key)
		switch {
		case index == int64(len(parent.items)):
			parent.items = append(parent.items, value)
		case index >= 0 && index < int64(len(parent.items)):
			parent.items[index] = value
		default:
			return mutationError("index " + key.Repr() + " out of range")
		}
		return nil
	case parent != nil && parent.isObject && key.IsString():
		if _, ok := parent.members[key.Text()]; !ok {
			parent.keys = append(parent.keys, key.Text())
		}
		parent.members[key.Text()] = value
		return nil
	}
	return mutationError("cannot set " + key.Repr())
}

// isIntLike reports whether Python's isinstance(value, int) holds: an
// integer or a boolean.
func isIntLike(value pyjson.Value) bool {
	_, ok := pyInt(value)
	return ok
}

// remove deletes parent[key] when it is there, as Python's del does; a key
// that is not there, or a parent that cannot lose one, changes nothing.
func remove(parent *node, key pyjson.Value) {
	switch {
	case parent == nil:
	case parent.isObject && key.IsString():
		if _, ok := parent.members[key.Text()]; ok {
			delete(parent.members, key.Text())
			parent.keys = slices.DeleteFunc(parent.keys, func(each string) bool { return each == key.Text() })
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
func applyMutation(state *node, record pyjson.Value) (*node, error) {
	kind := record.Get("kind")
	if equalsInt(kind, 0) {
		return newNode(record.Get("v")), nil
	}
	pathValue := record.Get("k")
	path := pathValue.Items()
	if !pathValue.IsArray() || (len(path) == 0 && !equalsInt(kind, 2)) {
		return state, mutationError("bad path " + pathValue.Repr())
	}
	target := state
	if len(path) > 0 {
		var err error
		if target, err = keyedMutation(state, kind, path, record.Get("v")); target == nil || err != nil {
			return state, err
		}
	}
	if target == nil || !target.isArray {
		return state, mutationError("push to non-list at " + pathValue.Repr())
	}
	if start, ok := pyInt(record.Get("i")); ok {
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
func keyedMutation(state *node, kind pyjson.Value, path []pyjson.Value, value pyjson.Value) (*node, error) {
	parent, err := walk(state, path[:len(path)-1])
	if err != nil {
		return nil, err
	}
	key := path[len(path)-1]
	switch {
	case equalsInt(kind, 1):
		return nil, set(parent, key, newNode(value))
	case equalsInt(kind, 3):
		remove(parent, key)
		return nil, nil
	case !equalsInt(kind, 2):
		return nil, mutationError("unknown kind " + kind.Repr())
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

// sliceStart returns where Python's target[start:] begins in a list of size
// items.
func sliceStart(start int64, size int) int {
	if start < 0 {
		start = max(start+int64(size), 0)
	}
	return int(min(start, int64(size)))
}
