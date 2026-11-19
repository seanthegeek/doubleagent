package analyze

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseTimelineReadsEveryOption(t *testing.T) {
	both := InputOptions{Input: "in"}
	tests := []struct {
		name string
		args []string
		want TimelineOptions
	}{
		{"the input and the output directory follow the options",
			[]string{"--since", "2026-10-01", "in", "out"},
			TimelineOptions{InputOptions: both, Output: "out", Since: "2026-10-01"}},
		{"no options", []string{"in", "out"}, TimelineOptions{InputOptions: both, Output: "out"}},
		{"repeatable options keep every value in order",
			[]string{"--agent", "aider", "--agent", "pi", "--match", "a", "--match=b", "in", "out"},
			TimelineOptions{InputOptions: both, Output: "out",
				Agents: []string{"aider", "pi"}, Match: []string{"a", "b"}}},
		{"flags and the shared input options are read",
			[]string{"--include-thinking", "--keep-undated", "-i", "--host", "h1", "--user", "u",
				"--work-dir", "w", "--keep-extracted", "--since", "2026", "--until", "now", "in", "out"},
			TimelineOptions{InputOptions: InputOptions{Input: "in", Host: "h1", User: "u",
				WorkDir: "w", KeepExtracted: true}, Output: "out",
				IncludeThinking: true, KeepUndated: true, IgnoreCase: true, Since: "2026",
				Until: "now"}},
		{"--ignore-case is -i", []string{"--ignore-case", "in", "out"},
			TimelineOptions{InputOptions: both, Output: "out", IgnoreCase: true}},
		{"a switch takes =false", []string{"-i", "--ignore-case=false", "in", "out"},
			TimelineOptions{InputOptions: both, Output: "out"}},
		{"a value may start with a dash or hold a space",
			[]string{"--match", "-1", "--match", "-a b", "--match", "--x", "in", "out"},
			TimelineOptions{InputOptions: both, Output: "out", Match: []string{"-1", "-a b", "--x"}}},
		{"after -- every argument is a positional", []string{"--", "-in", "-out"},
			TimelineOptions{InputOptions: InputOptions{Input: "-in"}, Output: "-out"}},
		{"a lone dash is a positional", []string{"-", "out"},
			TimelineOptions{InputOptions: InputOptions{Input: "-"}, Output: "out"}},
		{"-- may be a value after =", []string{"--match=--", "in", "out"},
			TimelineOptions{InputOptions: both, Output: "out", Match: []string{"--"}}},
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

// refusedCommandLine is a command line a command refuses, with the first
// line of its usage error after the command's name.
type refusedCommandLine struct {
	name    string
	command string
	args    []string
	want    string
}

// refusedCommandLines returns the command lines the analysis commands
// refuse.
func refusedCommandLines() []refusedCommandLine {
	return []refusedCommandLine{
		{"timeline needs its output directory", "timeline", []string{"in"},
			"missing argument: output directory"},
		{"an empty output directory is none", "timeline", []string{"in", ""},
			"missing argument: output directory"},
		{"timeline needs both", "timeline", nil, "missing argument: input, output directory"},
		{"an empty input of timeline is refused", "timeline", []string{"", "out"}, emptyInput},
		{"an empty input of detect is refused", "detect", []string{""}, emptyInput},
		{"an empty input of fleet is refused", "fleet", []string{"a", ""}, emptyInput},
		{"an empty work directory is refused", "detect", []string{"--work-dir=", "in"},
			`invalid value "" for flag -work-dir: an empty directory name`},
		{"timeline takes two arguments", "timeline", []string{"in", "out", "extra"},
			"unexpected argument: extra"},
		{"timeline names every extra argument", "timeline", []string{"in", "out", "a", "b"},
			"unexpected argument: a b"},
		{"-o is not an option of timeline", "timeline", []string{"-o", "out", "in"},
			"flag provided but not defined: -o"},
		{"--output is not an option of timeline", "timeline", []string{"--output", "out", "in"},
			"flag provided but not defined: -output"},
		{"--output= is not an option of timeline", "timeline", []string{"--output=out", "in"},
			"flag provided but not defined: -output"},
		{"an option's value is missing at the end", "timeline", []string{"--since"},
			"flag needs an argument: -since"},
		{"an unknown option is refused", "detect", []string{"--bogus", "in"},
			"flag provided but not defined: -bogus"},
		{"a short option with its value attached is refused", "timeline", []string{"-oout", "in", "out"},
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
		{"timeline with options after the input", "timeline", []string{"in", "out", "--since", "2026-10-01"},
			"options go before the input; --since is after it"},
		{"timeline with --help after the input", "timeline", []string{"in", "out", "--help"},
			"options go before the input; --help is after it"},
		{"timeline with an unknown option after the input", "timeline", []string{"in", "out", "--bogus"},
			"options go before the input; --bogus is after it"},
		{"a value -- does not end the options", "timeline", []string{"--since", "--", "in", "out", "--json"},
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
}

func TestParseRefusesWhatTheCommandDoesNotTake(t *testing.T) {
	for _, test := range refusedCommandLines() {
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
	detect, help, err := ParseDetect([]string{"--files", "--json", "--work-dir=w", "in"})
	want := DetectOptions{InputOptions: InputOptions{Input: "in", WorkDir: "w"}, JSON: true,
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
	if !takesValue("timeline", "since") || !takesValue("timeline", "match") || !takesValue("detect", "work-dir") {
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
	for _, args := range [][]string{{"-h"}, {"--help"}, {"-help"}, {"--since", "x", "-h", "in", "out"}} {
		if _, help, err := ParseTimeline(args); !help || err != nil {
			t.Errorf("%q: help %v, error %v", args, help, err)
		}
	}
}

func TestAMistakeAmongTheOptionsBeatsHelp(t *testing.T) {
	for _, args := range [][]string{{"--bogus", "--help"}, {"-h", "--bogus"}, {"-h", "-o", "x"}, {"-h", "in", "--json"}} {
		if _, help, err := ParseTimeline(args); help || err == nil {
			t.Errorf("%q: help %v, error %v; want a usage error", args, help, err)
		}
	}
}

func TestEachHelpGivesEveryExitStatus(t *testing.T) {
	for _, command := range []string{"detect", "timeline", "fleet"} {
		t.Run(command, func(t *testing.T) {
			help := strings.Join(strings.Fields(usageOf(command)), " ")
			for _, status := range []string{"0 when", "1 when", "2 on an error", "130 when interrupted"} {
				if !strings.Contains(help, status) {
					t.Errorf("the help does not give %q", status)
				}
			}
		})
	}
}
