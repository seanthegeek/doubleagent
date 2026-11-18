package analyze

import (
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
)

// UsageError is a command-line mistake: an unknown option, an option without
// its value, a missing or extra input. It is printed as collect prints its
// own: the message on one line, then the command's usage text; it gives
// exit code 2.
type UsageError struct {
	Command string
	Message string
}

// Error returns the message.
func (e *UsageError) Error() string {
	return e.Message
}

// Print writes the message, then the command's usage text, to w.
func (e *UsageError) Print(w io.Writer) {
	fmt.Fprintln(w, e.Message)
	fmt.Fprint(w, usageOf(e.Command))
}

// commandLine is the flag set of one command and the rules flag cannot
// state, checked once the line is read: how many inputs the command takes
// (at least one when needsInput is set, at most one unless many is) and the
// option one of whose names must be given. Parse prints nothing: parse words
// the error and the caller prints it with the command's documented usage
// text, which flag's own listing is not.
type commandLine struct {
	command    string
	set        *flag.FlagSet
	help       bool
	required   []string
	needsInput bool
	many       bool
}

// newCommandLine returns the command line of command with -h and --help.
func newCommandLine(command string) *commandLine {
	line := &commandLine{command: command, set: flag.NewFlagSet(command, flag.ContinueOnError)}
	line.set.SetOutput(io.Discard)
	line.set.Usage = func() {}
	line.switchOption(&line.help, "h", "help")
	return line
}

// stringOption registers target under each of names.
func (c *commandLine) stringOption(target *string, names ...string) {
	for _, name := range names {
		c.set.StringVar(target, name, *target, "")
	}
}

// listOption registers target under each of names; every use adds a value.
func (c *commandLine) listOption(target *[]string, names ...string) {
	for _, name := range names {
		c.set.Func(name, "", func(value string) error {
			*target = append(*target, value)
			return nil
		})
	}
}

// switchOption registers the boolean target under each of names.
func (c *commandLine) switchOption(target *bool, names ...string) {
	for _, name := range names {
		c.set.BoolVar(target, name, false, "")
	}
}

// usageError returns the usage error of message, worded as collect words its
// own: the binary and command name first.
func (c *commandLine) usageError(message string) *UsageError {
	return &UsageError{c.command, "doubleagent " + c.command + ": " + message}
}

// parse reads args and returns the inputs, and whether the help was asked
// for, which is answered before the inputs are checked.
func (c *commandLine) parse(args []string) ([]string, bool, error) {
	inputs, err := parseWithInputs(c.set, args)
	if err != nil {
		return nil, false, c.usageError(err.Error())
	}
	if c.help {
		return nil, true, nil
	}
	return inputs, false, c.check(inputs)
}

// parseWithInputs parses args with set and returns the arguments that are
// not options, wherever they stand. flag stops at the first such argument,
// and the documented command lines put options after the input (timeline
// INPUT -o DIR), so each stop takes one input and parses the rest again.
// Everything after the first "--" is an input; cutting the line there before
// flag reads it also keeps "--" from being taken as an option's value.
func parseWithInputs(set *flag.FlagSet, args []string) ([]string, error) {
	var inputs, afterEnd []string
	if end := slices.Index(args, "--"); end >= 0 {
		args, afterEnd = args[:end], args[end+1:]
	}
	for {
		if err := set.Parse(args); err != nil {
			return nil, err
		}
		if set.NArg() == 0 {
			return append(inputs, afterEnd...), nil
		}
		inputs = append(inputs, set.Arg(0))
		args = set.Args()[1:]
	}
}

// check returns the usage error of a line that misses an input or the
// required option, or has inputs the command does not take.
func (c *commandLine) check(inputs []string) error {
	if missing := c.missing(inputs); len(missing) > 0 {
		return c.usageError("missing argument: " + strings.Join(missing, ", "))
	}
	var extras []string
	switch {
	case !c.needsInput:
		extras = inputs
	case !c.many && len(inputs) > 1:
		extras = inputs[1:]
	}
	if len(extras) > 0 {
		return c.usageError("unexpected argument: " + strings.Join(extras, " "))
	}
	return nil
}

// missing returns what the line lacks: "input" when the command needs one
// and none was given, and the required option's spellings when none of its
// names was given.
func (c *commandLine) missing(inputs []string) []string {
	var missing []string
	if c.needsInput && len(inputs) == 0 {
		missing = append(missing, "input")
	}
	given := map[string]bool{}
	c.set.Visit(func(option *flag.Flag) { given[option.Name] = true })
	if len(c.required) > 0 && !slices.ContainsFunc(c.required, func(name string) bool { return given[name] }) {
		var spellings []string
		for _, name := range c.required {
			spellings = append(spellings, spelling(name))
		}
		missing = append(missing, strings.Join(spellings, "/"))
	}
	return missing
}

// spelling returns the option name as the usage text spells it: one dash
// before a one-letter name, two before a longer one.
func spelling(name string) string {
	if len(name) == 1 {
		return "-" + name
	}
	return "--" + name
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
	line := newCommandLine(command)
	line.switchOption(&options.KeepExtracted, "keep-extracted")
	line.stringOption(&options.Host, "host")
	line.stringOption(&options.User, "user")
	line.set.Func("work-dir", "", func(value string) error {
		options.WorkDir, options.workDirSet = value, true
		return nil
	})
	line.needsInput = true
	return line
}

// ParseDetect reads the arguments after `detect`. It reports whether the
// help was asked for.
func ParseDetect(args []string) (DetectOptions, bool, error) {
	var options DetectOptions
	line := inputLine("detect", &options.InputOptions)
	line.switchOption(&options.JSON, "json")
	line.switchOption(&options.Files, "files")
	inputs, help, err := line.parse(args)
	if help || err != nil {
		return options, help, err
	}
	options.Input = inputs[0]
	return options, false, nil
}

// ParseTimeline reads the arguments after `timeline`. It reports whether
// the help was asked for.
func ParseTimeline(args []string) (TimelineOptions, bool, error) {
	var options TimelineOptions
	line := inputLine("timeline", &options.InputOptions)
	line.stringOption(&options.Output, "o", "output")
	line.listOption(&options.Agents, "agent")
	line.stringOption(&options.Since, "since")
	line.stringOption(&options.Until, "until")
	line.listOption(&options.Match, "match")
	line.switchOption(&options.IncludeThinking, "include-thinking")
	line.switchOption(&options.KeepUndated, "keep-undated")
	line.switchOption(&options.IgnoreCase, "i", "ignore-case")
	line.required = []string{"o", "output"}
	inputs, help, err := line.parse(args)
	if help || err != nil {
		return options, help, err
	}
	options.Input = inputs[0]
	return options, false, nil
}

// ParseFleet reads the arguments after `fleet`. It reports whether the help
// was asked for.
func ParseFleet(args []string) (FleetOptions, bool, error) {
	options := FleetOptions{Output: DefaultFleetName}
	line := newCommandLine("fleet")
	line.stringOption(&options.Output, "o", "output")
	line.needsInput, line.many = true, true
	inputs, help, err := line.parse(args)
	options.Inputs = inputs
	return options, help, err
}

// ParseCatalog reads the arguments after `catalog`. It reports whether the
// help was asked for.
func ParseCatalog(args []string) (CatalogOptions, bool, error) {
	var options CatalogOptions
	line := newCommandLine("catalog")
	line.switchOption(&options.Agents, "agents")
	_, help, err := line.parse(args)
	return options, help, err
}
