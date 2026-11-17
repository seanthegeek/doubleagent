package analyze

import (
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
)

// UsageError is a command-line mistake: an unknown option, an option without
// its value, a missing or extra input. It is printed with the command's usage
// text and gives exit code 2.
type UsageError struct {
	Command string
	Message string
}

// Error returns the message.
func (e *UsageError) Error() string {
	return e.Message
}

// Print writes the command's usage text, then the message, to W.
func (e *UsageError) Print(w io.Writer) {
	fmt.Fprint(w, usageOf(e.Command))
	fmt.Fprintf(w, "doubleagent %s: error: %s\n", e.Command, e.Message)
}

// commandLine describes the options and inputs of one command for parse:
// the options that take no value, those that take one, the option that must
// be given (its spellings), and how many inputs it takes: inputs, at least
// one when needsInput is set, at most one unless many is.
type commandLine struct {
	command    string
	flags      map[string]*bool
	values     map[string]func(string)
	required   []string
	inputs     []string
	needsInput bool
	many       bool
}

// negativeNumber is an argument that reads as a negative number, which a
// value may be although it starts with a dash.
var negativeNumber = regexp.MustCompile(`^-[0-9]+\z|^-[0-9]*\.[0-9]+\z`)

// parse reads args into the command line's targets. It reports whether the
// help was asked for, which ends the reading at once. An option is spelled in
// full, its value either the next argument or after `=`; `--` makes every
// later argument an input.
func (c *commandLine) parse(args []string) (bool, error) {
	seen := map[string]bool{}
	var extras []string
	for i := 0; i < len(args); i++ {
		argument := args[i]
		switch {
		case argument == "--":
			c.inputs = append(c.inputs, args[i+1:]...)
			i = len(args)
		case argument == "-h" || argument == "--help":
			return true, nil
		case c.isOption(argument):
			consumed, err := c.option(args[i:], seen)
			if err != nil {
				return false, err
			}
			if consumed < 0 {
				extras = append(extras, argument)
				consumed = 0
			}
			i += consumed
		default:
			c.inputs = append(c.inputs, argument)
		}
	}
	return false, c.check(seen, extras)
}

// isOption reports whether an argument stands for an option rather than a
// value: it starts with a dash and is not a lone dash, a negative number, or
// text holding a space that names no option.
func (c *commandLine) isOption(argument string) bool {
	if !strings.HasPrefix(argument, "-") || argument == "-" {
		return false
	}
	name, _, _ := strings.Cut(argument, "=")
	if c.known(name) || c.known(argument) {
		return true
	}
	return !negativeNumber.MatchString(argument) && !strings.Contains(argument, " ")
}

// known reports whether name is one of the command's options.
func (c *commandLine) known(name string) bool {
	_, flag := c.flags[name]
	_, value := c.values[name]
	return flag || value
}

// option reads the option at args[0] and returns how many further arguments
// it took as its value, or -1 for an option the command does not have.
func (c *commandLine) option(args []string, seen map[string]bool) (int, error) {
	name, attached, hasAttached := strings.Cut(args[0], "=")
	if !c.known(name) && len(name) > 2 && !strings.HasPrefix(name, "--") {
		// A short option with its value attached, as in -oDIR.
		name, attached, hasAttached = args[0][:2], args[0][2:], true
	}
	if flag, ok := c.flags[name]; ok {
		if hasAttached {
			return 0, &UsageError{c.command, fmt.Sprintf("argument %s: ignored explicit argument %s",
				name, quoted(attached))}
		}
		*flag = true
		return 0, nil
	}
	setter, ok := c.values[name]
	if !ok {
		return -1, nil
	}
	seen[name] = true
	if hasAttached {
		setter(attached)
		return 0, nil
	}
	if len(args) < 2 || c.isOption(args[1]) {
		return 0, &UsageError{c.command, fmt.Sprintf("argument %s: expected one argument", name)}
	}
	setter(args[1])
	return 1, nil
}

// check returns the usage error of a line that misses an input or the
// required option, or has arguments the command does not take.
func (c *commandLine) check(seen map[string]bool, extras []string) error {
	var missing []string
	if c.needsInput && len(c.inputs) == 0 {
		missing = append(missing, "input")
	}
	if len(c.required) > 0 && !slices.ContainsFunc(c.required, func(name string) bool {
		return seen[name]
	}) {
		missing = append(missing, strings.Join(c.required, "/"))
	}
	if len(missing) > 0 {
		return &UsageError{c.command, "the following arguments are required: " +
			strings.Join(missing, ", ")}
	}
	switch {
	case !c.needsInput:
		extras = append(extras, c.inputs...)
	case !c.many && len(c.inputs) > 1:
		extras = append(extras, c.inputs[1:]...)
	}
	if len(extras) > 0 {
		return &UsageError{c.command, "unrecognized arguments: " + strings.Join(extras, " ")}
	}
	return nil
}

// quoted returns text in single quotes, as the usage messages show a value.
func quoted(text string) string {
	return "'" + text + "'"
}

// InputOptions are the options detect and timeline share: the input and how
// it is opened.
type InputOptions struct {
	Input         string
	Host          string
	User          string
	WorkDir       string
	KeepExtracted bool
	// workDirSet tells an empty --work-dir, the current directory, from
	// none, a temporary directory.
	workDirSet bool
}

// DetectOptions are the options of detect.
type DetectOptions struct {
	InputOptions
	JSON  bool
	Files bool
}

// TimelineOptions are the options of timeline.
type TimelineOptions struct {
	InputOptions
	Output          string
	IncludeThinking bool
	Agents          []string
	Since           string
	Until           string
	KeepUndated     bool
	Match           []string
	IgnoreCase      bool
}

// FleetOptions are the options of fleet: the saved inventory outputs and
// the CSV to write.
type FleetOptions struct {
	Inputs []string
	Output string
}

// CatalogOptions are the options of catalog.
type CatalogOptions struct {
	Agents bool
}

// inputLine returns the command line of the options detect and timeline
// share, reading into options.
func inputLine(command string, options *InputOptions) *commandLine {
	set := func(target *string) func(string) { return func(value string) { *target = value } }
	return &commandLine{
		command: command,
		flags:   map[string]*bool{"--keep-extracted": &options.KeepExtracted},
		values: map[string]func(string){"--host": set(&options.Host), "--user": set(&options.User),
			"--work-dir": func(value string) { options.WorkDir, options.workDirSet = value, true }},
		needsInput: true,
	}
}

// ParseDetect reads the arguments after `detect`. It reports whether the
// help was asked for.
func ParseDetect(args []string) (DetectOptions, bool, error) {
	var options DetectOptions
	line := inputLine("detect", &options.InputOptions)
	line.flags["--json"] = &options.JSON
	line.flags["--files"] = &options.Files
	help, err := line.parse(args)
	if help || err != nil {
		return options, help, err
	}
	options.Input = line.inputs[0]
	return options, false, nil
}

// ParseTimeline reads the arguments after `timeline`. It reports whether
// the help was asked for.
func ParseTimeline(args []string) (TimelineOptions, bool, error) {
	var options TimelineOptions
	line := inputLine("timeline", &options.InputOptions)
	output := func(value string) { options.Output = value }
	line.values["-o"], line.values["--output"] = output, output
	line.values["--agent"] = func(value string) { options.Agents = append(options.Agents, value) }
	line.values["--since"] = func(value string) { options.Since = value }
	line.values["--until"] = func(value string) { options.Until = value }
	line.values["--match"] = func(value string) { options.Match = append(options.Match, value) }
	line.flags["--include-thinking"] = &options.IncludeThinking
	line.flags["--keep-undated"] = &options.KeepUndated
	line.flags["-i"], line.flags["--ignore-case"] = &options.IgnoreCase, &options.IgnoreCase
	line.required = []string{"-o", "--output"}
	help, err := line.parse(args)
	if help || err != nil {
		return options, help, err
	}
	options.Input = line.inputs[0]
	return options, false, nil
}

// ParseFleet reads the arguments after `fleet`. It reports whether the help
// was asked for.
func ParseFleet(args []string) (FleetOptions, bool, error) {
	options := FleetOptions{Output: DefaultFleetName}
	output := func(value string) { options.Output = value }
	line := &commandLine{command: "fleet", flags: map[string]*bool{},
		values:     map[string]func(string){"-o": output, "--output": output},
		needsInput: true, many: true}
	help, err := line.parse(args)
	options.Inputs = line.inputs
	return options, help, err
}

// ParseCatalog reads the arguments after `catalog`. It reports whether the
// help was asked for.
func ParseCatalog(args []string) (CatalogOptions, bool, error) {
	var options CatalogOptions
	line := &commandLine{command: "catalog", flags: map[string]*bool{"--agents": &options.Agents},
		values: map[string]func(string){}}
	help, err := line.parse(args)
	return options, help, err
}
