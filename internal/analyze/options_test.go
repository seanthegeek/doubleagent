package analyze

import (
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
		{"-- may be -o's value", []string{"-o", "--", "in"},
			TimelineOptions{InputOptions: InputOptions{Input: "in"}, Output: "--"}},
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

// parseError returns the error each command's parse function gives for
// args.
func parseError(command string, args []string) error {
	var err error
	switch command {
	case "detect":
		_, _, err = ParseDetect(args)
	case "timeline":
		_, _, err = ParseTimeline(args)
	case "fleet":
		_, _, err = ParseFleet(args)
	case "catalog":
		_, _, err = ParseCatalog(args)
	}
	return err
}

func TestParseRefusesWhatTheCommandDoesNotTake(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    string
	}{
		{"timeline needs its output", "timeline", []string{"in"}, "missing argument: -o/--output"},
		{"an empty output is no output", "timeline", []string{"-o", "", "in"}, "missing argument: -o/--output"},
		{"timeline needs its input", "timeline", []string{"-o", "x"}, "missing argument: input"},
		{"timeline needs both", "timeline", nil, "missing argument: input, -o/--output"},
		{"an option's value is missing at the end", "timeline", []string{"-o"}, "flag needs an argument: -o"},
		{"an unknown option is refused", "detect", []string{"--bogus", "in"},
			"flag provided but not defined: -bogus"},
		{"a short option with its value attached is refused", "timeline", []string{"-oout", "in"},
			"flag provided but not defined: -oout"},
		{"a negative number is not an input", "detect", []string{"-1"}, "flag provided but not defined: -1"},
		{"--version is not an analysis option", "timeline", []string{"--version"},
			"flag provided but not defined: -version"},
		{"detect takes one input", "detect", []string{"a", "b"}, "unexpected argument: b"},
		{"detect names every extra input", "detect", []string{"a", "b", "c"}, "unexpected argument: b c"},
		{"an extra input after -- is refused", "detect", []string{"--", "a", "b"}, "unexpected argument: b"},
		{"a switch takes no value that is not a boolean", "detect", []string{"--json=yes", "in"},
			`invalid boolean value "yes" for -json: parse error`},
		{"fleet needs an input", "fleet", nil, "missing argument: input"},
		{"catalog takes no input", "catalog", []string{"x"}, "unexpected argument: x"},
		{"catalog refuses an unknown option", "catalog", []string{"--bogus"},
			"flag provided but not defined: -bogus"},
		{"detect with a switch after the input", "detect", []string{"in", "--json"},
			"options go before the input; --json is after it"},
		{"detect with a value option after the input", "detect", []string{"in", "--host", "h"},
			"options go before the input; --host is after it"},
		{"timeline with options after the input", "timeline", []string{"in", "--since", "2026-10-01", "-o", "out"},
			"options go before the input; --since is after it"},
		{"timeline with --help after the input", "timeline", []string{"-o", "x", "in", "--help"},
			"options go before the input; --help is after it"},
		{"timeline with an unknown option after the input", "timeline", []string{"-o", "x", "in", "--bogus"},
			"options go before the input; --bogus is after it"},
		{"-o's value -- does not end the options", "timeline", []string{"-o", "--", "in", "--json"},
			"options go before the input; --json is after it"},
		{"fleet with -o between its inputs", "fleet", []string{"a", "-o", "x.csv", "b"},
			"options go before the input; -o is after it"},
		{"fleet with an option after -o's value --", "fleet", []string{"-o", "--", "a", "-c"},
			"options go before the input; -c is after it"},
		{"fleet with -- after its inputs", "fleet", []string{"-o", "x.csv", "a", "b", "--", "-c"},
			"options go before the input; -- is after it"},
		{"catalog with --agents after an argument", "catalog", []string{"x", "--agents"},
			"options go before the input; --agents is after it"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := parseError(test.command, test.args)
			if err == nil {
				t.Fatal("no error")
			}
			want := "doubleagent " + test.command + ": " + test.want
			if got, _, _ := strings.Cut(err.Error(), "\n"); got != want {
				t.Errorf("message %q, want %q", got, want)
			}
		})
	}
}

func TestAUsageErrorEndsWithTheUsage(t *testing.T) {
	want := "doubleagent fleet: missing argument: input\n" + strings.TrimSuffix(fleetUsage, "\n")
	if err := parseError("fleet", nil); err == nil || err.Error() != want {
		t.Errorf("error %q, want %q", err, want)
	}
}

func TestParseFleetAndDetect(t *testing.T) {
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
	for _, args := range [][]string{{"--bogus", "--help"}, {"-h", "--bogus"}, {"-h", "-o"}, {"-h", "in", "--json"}} {
		if _, help, err := ParseTimeline(args); help || err == nil {
			t.Errorf("%q: help %v, error %v; want a usage error", args, help, err)
		}
	}
}
