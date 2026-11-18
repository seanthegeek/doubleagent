package analyze

// firstMet keeps values in the order their keys were first met, beside a
// map from each key to its place, so finding a key's value never searches
// the list: the homes, users, agents and projects of an input grow with the
// evidence, and a search per artifact would grow with its square.
type firstMet[K comparable, V any] struct {
	values []V
	places map[K]int
}

// entry returns the value of key, after appending fresh as its value when key
// is new. The pointer is good until the next call that appends.
func (f *firstMet[K, V]) entry(key K, fresh V) *V {
	place, ok := f.places[key]
	if !ok {
		if f.places == nil {
			f.places = map[K]int{}
		}
		place = len(f.values)
		f.places[key] = place
		f.values = append(f.values, fresh)
	}
	return &f.values[place]
}

// get returns the value of key and true, or the zero value and false when
// key was never met.
func (f *firstMet[K, V]) get(key K) (V, bool) {
	place, ok := f.places[key]
	if !ok {
		var zero V
		return zero, false
	}
	return f.values[place], true
}
