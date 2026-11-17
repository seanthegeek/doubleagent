package analyze

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseTimelineReadsEveryOption(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want TimelineOptions
	}{
		{"the input may follow the options", []string{"-o", "out", "in"},
			TimelineOptions{InputOptions: InputOptions{Input: "in"}, Output: "out"}},
		{"repeatable options keep every value in order",
			[]string{"in", "--agent", "aider", "--output=out", "--agent", "pi", "--match", "a",
				"--match=b"},
			TimelineOptions{InputOptions: InputOptions{Input: "in"}, Output: "out",
				Agents: []string{"aider", "pi"}, Match: []string{"a", "b"}}},
		{"flags and the shared input options are read",
			[]string{"--include-thinking", "--keep-undated", "-i", "--host", "h1", "--user", "u",
				"--work-dir", "w", "--keep-extracted", "-oout", "in", "--since", "2026",
				"--until", "now"},
			TimelineOptions{InputOptions: InputOptions{Input: "in", Host: "h1", User: "u",
				WorkDir: "w", KeepExtracted: true, workDirSet: true}, Output: "out",
				IncludeThinking: true, KeepUndated: true, IgnoreCase: true, Since: "2026",
				Until: "now"}},
		{"a value may be a negative number or hold a space",
			[]string{"in", "-o", "out", "--match", "-1", "--match", "-a b"},
			TimelineOptions{InputOptions: InputOptions{Input: "in"}, Output: "out",
				Match: []string{"-1", "-a b"}}},
		{"after -- every argument is an input", []string{"-o", "out", "--", "-in"},
			TimelineOptions{InputOptions: InputOptions{Input: "-in"}, Output: "out"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, help, err := ParseTimeline(test.args)
			if err != nil || help {
				t.Fatalf("error %v, help %v", err, help)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %+v\nwant %+v", got, test.want)
			}
		})
	}
}

func TestParseRefusesWhatTheCommandDoesNotTake(t *testing.T) {
	tests := []struct {
		name  string
		parse func() error
		want  string
	}{
		{"timeline needs its output", func() error { _, _, err := ParseTimeline([]string{"in"}); return err },
			"the following arguments are required: -o/--output"},
		{"timeline needs its input", func() error { _, _, err := ParseTimeline([]string{"-o", "x"}); return err },
			"the following arguments are required: input"},
		{"an option's value is missing at the end",
			func() error { _, _, err := ParseTimeline([]string{"in", "-o"}); return err },
			"argument -o: expected one argument"},
		{"an option's value may not look like an option",
			func() error { _, _, err := ParseTimeline([]string{"in", "-o", "--agent", "x"}); return err },
			"argument -o: expected one argument"},
		{"an unknown option is refused",
			func() error { _, _, err := ParseDetect([]string{"in", "--bogus"}); return err },
			"unrecognized arguments: --bogus"},
		{"detect takes one input", func() error { _, _, err := ParseDetect([]string{"a", "b"}); return err },
			"unrecognized arguments: b"},
		{"a flag takes no value",
			func() error { _, _, err := ParseDetect([]string{"in", "--json=yes"}); return err },
			"argument --json: ignored explicit argument 'yes'"},
		{"fleet needs an input", func() error { _, _, err := ParseFleet(nil); return err },
			"the following arguments are required: input"},
		{"catalog takes no input", func() error { _, _, err := ParseCatalog([]string{"x"}); return err },
			"unrecognized arguments: x"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.parse()
			var usageError *UsageError
			if !errors.As(err, &usageError) || usageError.Message != test.want {
				t.Errorf("got %v, want the usage error %q", err, test.want)
			}
		})
	}
}

func TestParseFleetAndDetect(t *testing.T) {
	fleet, help, err := ParseFleet([]string{"a", "-o", "x.csv", "b"})
	if err != nil || help || !reflect.DeepEqual(fleet, FleetOptions{Inputs: []string{"a", "b"},
		Output: "x.csv"}) {
		t.Errorf("fleet: %+v, %v, %v", fleet, help, err)
	}
	if fleet, _, _ = ParseFleet([]string{"a"}); fleet.Output != DefaultFleetName {
		t.Errorf("fleet's default output is %q", fleet.Output)
	}
	detect, help, err := ParseDetect([]string{"--files", "in", "--json", "--work-dir="})
	want := DetectOptions{InputOptions: InputOptions{Input: "in", workDirSet: true}, JSON: true,
		Files: true}
	if err != nil || help || !reflect.DeepEqual(detect, want) {
		t.Errorf("detect: %+v, %v, %v", detect, help, err)
	}
}

func TestHelpEndsTheLineBeforeAnyMistakeAfterIt(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--bogus", "--help"}, {"in", "-h", "-o"}} {
		if _, help, err := ParseTimeline(args); !help || err != nil {
			t.Errorf("%q: help %v, error %v", args, help, err)
		}
	}
}

func TestUsageErrorPrintsTheUsageThenTheMessage(t *testing.T) {
	var out strings.Builder
	(&UsageError{Command: "fleet", Message: "boom"}).Print(&out)
	if !strings.HasPrefix(out.String(), "usage: doubleagent fleet ") ||
		!strings.HasSuffix(out.String(), "\ndoubleagent fleet: error: boom\n") {
		t.Errorf("printed %q", out.String())
	}
}
