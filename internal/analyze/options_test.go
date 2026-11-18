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
		{"the input follows the options", []string{"--since", "2026-10-01", "-o", "out", "in"},
			TimelineOptions{InputOptions: InputOptions{Input: "in"}, Output: "out", Since: "2026-10-01"}},
		{"--output is -o", []string{"--output", "out", "in"},
			TimelineOptions{InputOptions: InputOptions{Input: "in"}, Output: "out"}},
		{"-output with one dash is --output", []string{"-output", "out", "in"},
			TimelineOptions{InputOptions: InputOptions{Input: "in"}, Output: "out"}},
		{"-o takes a value after =", []string{"-o=out", "in"},
			TimelineOptions{InputOptions: InputOptions{Input: "in"}, Output: "out"}},
		{"repeatable options keep every value in order",
			[]string{"--agent", "aider", "--output=out", "--agent", "pi", "--match", "a",
				"--match=b", "in"},
			TimelineOptions{InputOptions: InputOptions{Input: "in"}, Output: "out",
				Agents: []string{"aider", "pi"}, Match: []string{"a", "b"}}},
		{"flags and the shared input options are read",
			[]string{"--include-thinking", "--keep-undated", "-i", "--host", "h1", "--user", "u",
				"--work-dir", "w", "--keep-extracted", "-o", "out", "--since", "2026",
				"--until", "now", "in"},
			TimelineOptions{InputOptions: InputOptions{Input: "in", Host: "h1", User: "u",
				WorkDir: "w", KeepExtracted: true, workDirSet: true}, Output: "out",
				IncludeThinking: true, KeepUndated: true, IgnoreCase: true, Since: "2026",
				Until: "now"}},
		{"--ignore-case is -i", []string{"-o", "out", "--ignore-case", "in"},
			TimelineOptions{InputOptions: InputOptions{Input: "in"}, Output: "out", IgnoreCase: true}},
		{"a switch takes =false", []string{"-o", "out", "-i", "--ignore-case=false", "in"},
			TimelineOptions{InputOptions: InputOptions{Input: "in"}, Output: "out"}},
		{"a value may start with a dash or hold a space",
			[]string{"-o", "out", "--match", "-1", "--match", "-a b", "--match", "--x", "in"},
			TimelineOptions{InputOptions: InputOptions{Input: "in"}, Output: "out",
				Match: []string{"-1", "-a b", "--x"}}},
		{"after -- every argument is an input", []string{"-o", "out", "--", "-in"},
			TimelineOptions{InputOptions: InputOptions{Input: "-in"}, Output: "out"}},
		{"a lone dash is an input", []string{"-o", "out", "-"},
			TimelineOptions{InputOptions: InputOptions{Input: "-"}, Output: "out"}},
		{"-- may be a value after =", []string{"-o", "out", "--match=--", "in"},
			TimelineOptions{InputOptions: InputOptions{Input: "in"}, Output: "out", Match: []string{"--"}}},
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

// usageMessage returns the message of the usage error parse returns, or
// fails the test when it returns no usage error.
func usageMessage(t *testing.T, parse func() error) string {
	t.Helper()
	var usageError *UsageError
	if err := parse(); !errors.As(err, &usageError) {
		t.Fatalf("got %v, want a usage error", err)
	}
	return usageError.Message
}

func TestParseRefusesWhatTheCommandDoesNotTake(t *testing.T) {
	timeline := func(args ...string) func() error {
		return func() error { _, _, err := ParseTimeline(args); return err }
	}
	detect := func(args ...string) func() error {
		return func() error { _, _, err := ParseDetect(args); return err }
	}
	tests := []struct {
		name  string
		parse func() error
		want  string
	}{
		{"timeline needs its output", timeline("in"), "doubleagent timeline: missing argument: -o/--output"},
		{"timeline needs its input", timeline("-o", "x"), "doubleagent timeline: missing argument: input"},
		{"timeline needs both", timeline(), "doubleagent timeline: missing argument: input, -o/--output"},
		{"an option's value is missing at the end", timeline("-o"),
			"doubleagent timeline: flag needs an argument: -o"},
		{"an unknown option is refused", detect("--bogus", "in"),
			"doubleagent detect: flag provided but not defined: -bogus"},
		{"a short option with its value attached is refused", timeline("-oout", "in"),
			"doubleagent timeline: flag provided but not defined: -oout"},
		{"a negative number is not an input", detect("-1"),
			"doubleagent detect: flag provided but not defined: -1"},
		{"--version is not an analysis option", timeline("--version"),
			"doubleagent timeline: flag provided but not defined: -version"},
		{"detect takes one input", detect("a", "b"), "doubleagent detect: unexpected argument: b"},
		{"detect names every extra input", detect("a", "b", "c"),
			"doubleagent detect: unexpected argument: b c"},
		{"an extra input after -- is refused", detect("--", "a", "b"),
			"doubleagent detect: unexpected argument: b"},
		{"a switch takes no value that is not a boolean", detect("--json=yes", "in"),
			`doubleagent detect: invalid boolean value "yes" for -json: parse error`},
		{"fleet needs an input",
			func() error { _, _, err := ParseFleet(nil); return err }, "doubleagent fleet: missing argument: input"},
		{"catalog takes no input",
			func() error { _, _, err := ParseCatalog([]string{"x"}); return err },
			"doubleagent catalog: unexpected argument: x"},
		{"catalog refuses an unknown option",
			func() error { _, _, err := ParseCatalog([]string{"--bogus"}); return err },
			"doubleagent catalog: flag provided but not defined: -bogus"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := usageMessage(t, test.parse); got != test.want {
				t.Errorf("message %q, want %q", got, test.want)
			}
		})
	}
}

func TestAnOptionAfterTheInputIsAUsageError(t *testing.T) {
	parsers := map[string]func([]string) error{
		"detect":   func(args []string) error { _, _, err := ParseDetect(args); return err },
		"timeline": func(args []string) error { _, _, err := ParseTimeline(args); return err },
		"fleet":    func(args []string) error { _, _, err := ParseFleet(args); return err },
		"catalog":  func(args []string) error { _, _, err := ParseCatalog(args); return err },
	}
	tests := []struct {
		name    string
		command string
		args    []string
		option  string
	}{
		{"detect with a switch after the input", "detect", []string{"in", "--json"}, "--json"},
		{"detect with a value option after the input", "detect", []string{"in", "--host", "h"}, "--host"},
		{"timeline with its output after the input", "timeline",
			[]string{"in", "--since", "2026-10-01", "-o", "out"}, "--since"},
		{"timeline with --help after the input", "timeline", []string{"-o", "x", "in", "--help"}, "--help"},
		{"timeline with an unknown option after the input", "timeline", []string{"-o", "x", "in", "--bogus"},
			"--bogus"},
		{"fleet with -o between its inputs", "fleet", []string{"a", "-o", "x.csv", "b"}, "-o"},
		{"catalog with --agents after an argument", "catalog", []string{"x", "--agents"}, "--agents"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parse := parsers[test.command]
			got := usageMessage(t, func() error { return parse(test.args) })
			want := "doubleagent " + test.command + ": options go before the input; " + test.option +
				" is after it"
			if got != want {
				t.Errorf("message %q, want %q", got, want)
			}
		})
	}
}

func TestParseFleetAndDetect(t *testing.T) {
	if fleet, _, err := ParseFleet([]string{"-o", "x.csv", "a", "b", "--", "-c"}); err == nil {
		t.Errorf("fleet took -- after its inputs: %+v", fleet)
	}
	fleet, help, err := ParseFleet([]string{"-o", "x.csv", "--", "a", "-c"})
	if err != nil || help || !reflect.DeepEqual(fleet, FleetOptions{Inputs: []string{"a", "-c"},
		Output: "x.csv"}) {
		t.Errorf("fleet: %+v, %v, %v", fleet, help, err)
	}
	if fleet, _, _ = ParseFleet([]string{"a"}); fleet.Output != DefaultFleetName {
		t.Errorf("fleet's default output is %q", fleet.Output)
	}
	if fleet, _, _ = ParseFleet([]string{"--output=y.csv", "a"}); fleet.Output != "y.csv" {
		t.Errorf("fleet's --output= gave %q", fleet.Output)
	}
	detect, help, err := ParseDetect([]string{"--files", "--json", "--work-dir=", "in"})
	want := DetectOptions{InputOptions: InputOptions{Input: "in", workDirSet: true}, JSON: true,
		Files: true}
	if err != nil || help || !reflect.DeepEqual(detect, want) {
		t.Errorf("detect: %+v, %v, %v", detect, help, err)
	}
	catalog, help, err := ParseCatalog([]string{"-agents"})
	if err != nil || help || !catalog.Agents {
		t.Errorf("catalog: %+v, %v, %v", catalog, help, err)
	}
}

func TestFlagSetTellsWhichOptionsTakeAValue(t *testing.T) {
	takesValue := func(command, name string) bool {
		option := FlagSet(command).Lookup(name)
		if option == nil {
			t.Fatalf("%s has no option %s", command, name)
		}
		switchValue, ok := option.Value.(interface{ IsBoolFlag() bool })
		return !ok || !switchValue.IsBoolFlag()
	}
	if !takesValue("timeline", "o") || !takesValue("timeline", "match") || !takesValue("detect", "work-dir") {
		t.Error("a value option reads as a switch")
	}
	if takesValue("detect", "json") || takesValue("timeline", "i") || takesValue("catalog", "agents") {
		t.Error("a switch reads as a value option")
	}
	if FlagSet("collect") != nil {
		t.Error("collect is not an analysis command")
	}
}

func TestHelpIsAskedForAmongTheOptions(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"-help"}, {"-o", "x", "-h", "in"}} {
		if _, help, err := ParseTimeline(args); !help || err != nil {
			t.Errorf("%q: help %v, error %v", args, help, err)
		}
	}
}

func TestAMistakeAmongTheOptionsBeatsHelp(t *testing.T) {
	for _, args := range [][]string{{"--bogus", "--help"}, {"-h", "--bogus"}, {"-h", "-o"}} {
		if _, help, err := ParseTimeline(args); help || err == nil {
			t.Errorf("%q: help %v, error %v; want a usage error", args, help, err)
		}
	}
}

func TestUsageErrorPrintsTheMessageThenTheUsage(t *testing.T) {
	var out strings.Builder
	(&UsageError{Command: "fleet", Message: "boom"}).Print(&out)
	if out.String() != "boom\n"+fleetUsage {
		t.Errorf("printed %q", out.String())
	}
}
