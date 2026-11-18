//go:build parity

package replay

import (
	"flag"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/analyze"
)

// optionsFirst returns the arguments after an analysis command with every
// option, and the value it takes, moved ahead of the inputs, and how many
// options it moved (difference 121): the Python analyzer took options after
// its input, the binary takes them only before it. The binary's own flag
// set says which options take a value; an option it does not have is moved
// alone, so the binary still refuses it. A "--" in the recording is kept
// ahead of the inputs, so an input after it that starts with a dash stays
// an input. For timeline the recorded -o or --output (in any of its forms)
// is not an option of the binary: its value goes last, as the second
// argument.
func optionsFirst(command string, args []string) ([]string, int) {
	set := analyze.FlagSet(command)
	var options, inputs, output []string
	moved, ended := 0, false
	for i := 0; i < len(args); i++ {
		argument := args[i]
		switch {
		case argument == "--":
			inputs, ended = append(inputs, args[i+1:]...), true
			i = len(args)
		case len(argument) < 2 || argument[0] != '-':
			inputs = append(inputs, argument)
		case command == "timeline" && isOutputOption(argument):
			moved++
			if _, value, attached := strings.Cut(argument, "="); attached {
				output = append(output, value)
			} else if i+1 < len(args) {
				i++
				output = append(output, args[i])
			}
		default:
			if len(inputs) > 0 {
				moved++
			}
			options = append(options, argument)
			if takesValue(set, argument) && i+1 < len(args) {
				i++
				options = append(options, args[i])
			}
		}
	}
	if moved == 0 {
		return args, 0
	}
	if ended {
		options = append(options, "--")
	}
	return slices.Concat(options, inputs, output), moved
}

// isOutputOption reports whether argument is the Python analyzer's -o or
// --output option, with or without a value attached by "=".
func isOutputOption(argument string) bool {
	name, _, _ := strings.Cut(strings.TrimLeft(argument, "-"), "=")
	return name == "o" || name == "output"
}

// takesValue reports whether argument names an option of set that takes
// its value from the next argument: one set has, that is not a switch and
// has no value after "=".
func takesValue(set *flag.FlagSet, argument string) bool {
	name, _, attached := strings.Cut(strings.TrimLeft(argument, "-"), "=")
	option := set.Lookup(name)
	if attached || option == nil {
		return false
	}
	switchValue, ok := option.Value.(interface{ IsBoolFlag() bool })
	return !ok || !switchValue.IsBoolFlag()
}

func TestOptionsFirstMovesEachOptionWithItsValue(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    []string
		moved   int
	}{
		{"options already first are left alone", "timeline", []string{"--since", "2026", "in", "out"},
			[]string{"--since", "2026", "in", "out"}, 0},
		{"timeline's output option becomes the second argument", "timeline", []string{"-o", "out", "in"},
			[]string{"in", "out"}, 1},
		{"--output after the input becomes the second argument", "timeline",
			[]string{"in", "--output", "out", "--since", "2026-10-01"},
			[]string{"--since", "2026-10-01", "in", "out"}, 2},
		{"--output= becomes the second argument", "timeline", []string{"in", "--output=out", "-i"},
			[]string{"-i", "in", "out"}, 2},
		{"-o= becomes the second argument", "timeline", []string{"in", "-o=out"},
			[]string{"in", "out"}, 1},
		{"the output stays last behind --", "timeline", []string{"in", "-o", "out", "--", "-dash"},
			[]string{"--", "in", "-dash", "out"}, 1},
		{"a value option after the input moves with its value", "timeline",
			[]string{"in", "-o", "out", "--since", "2026-10-01"},
			[]string{"--since", "2026-10-01", "in", "out"}, 2},
		{"a switch after the input moves alone", "detect", []string{"in", "--json"},
			[]string{"--json", "in"}, 1},
		{"a value after = moves with its option", "timeline", []string{"in", "--match=a", "-i"},
			[]string{"--match=a", "-i", "in"}, 2},
		{"a value that starts with a dash stays the option's value", "timeline",
			[]string{"in", "-o", "out", "--match", "-x"},
			[]string{"--match", "-x", "in", "out"}, 2},
		{"inputs keep their order around a moved option", "fleet", []string{"a", "-o", "x.csv", "b"},
			[]string{"-o", "x.csv", "a", "b"}, 1},
		{"-- stays ahead of the inputs", "detect", []string{"in", "--json", "--", "-dash"},
			[]string{"--json", "--", "in", "-dash"}, 1},
		{"an unknown option moves alone", "detect", []string{"in", "--bogus", "x"},
			[]string{"--bogus", "in", "x"}, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, moved := optionsFirst(test.command, test.args)
			if !slices.Equal(got, test.want) || moved != test.moved {
				t.Errorf("got %q, %d moved; want %q, %d", got, moved, test.want, test.moved)
			}
		})
	}
}

func TestTheReorderedLineParsesAsTheRecordedOneMeant(t *testing.T) {
	args, _ := optionsFirst("timeline", []string{"in", "-o", "out", "--match", "-x", "--keep-undated"})
	options, help, err := analyze.ParseTimeline(args)
	if err != nil || help || options.Input != "in" || options.Output != "out" ||
		!slices.Equal(options.Match, []string{"-x"}) || !options.KeepUndated {
		t.Errorf("parsed %+v, help %v, error %v", options, help, err)
	}
}
