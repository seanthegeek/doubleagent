package stage

import "testing"

func TestPlacesTellAnotherEntryFromTheSameOne(t *testing.T) {
	places := NewPlaces()
	alice, other := [2]uint64{1, 10}, [2]uint64{1, 11}
	steps := []struct {
		name     string
		existed  bool
		identity [2]uint64
		want     bool
	}{
		{"a new place is filled", false, alice, true},
		{"the same entry staged again may fill it", true, alice, true},
		{"another entry may not", true, other, false},
		{"an entry with no identity may not", true, [2]uint64{}, false},
	}
	for _, step := range steps {
		if got := places.fill("/s/fs/Alice/x", step.existed, step.identity); got != step.want {
			t.Errorf("%s: fill = %v, want %v", step.name, got, step.want)
		}
	}
	if places.fill("/s/fs/unrecorded", true, alice) {
		t.Error("a place the stager did not record was written over")
	}
}
