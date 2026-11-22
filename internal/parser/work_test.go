package parser

import "testing"

func TestWorkCountsOnlyWhenATestTurnsItOn(t *testing.T) {
	tests := []struct {
		name string
		work Work
		want int
	}{
		{"the zero Work counts nothing", Work{}, 0},
		{"a counting Work counts every unit, through its copies too", CountingWork(), 5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			copied := test.work
			test.work.Add("reads", 2)
			copied.Add("reads", 3)
			copied.Add("writes", 7)
			if got := test.work.Count("reads"); got != test.want {
				t.Errorf("counted %d reads, want %d", got, test.want)
			}
		})
	}
}
