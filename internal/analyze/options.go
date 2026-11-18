package analyze

import (
	"errors"
	"flag"
	"io"
	"strings"
)

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

// newFlagSet returns the flag set of command with -h and --help, which set
// help. It prints nothing: a mistake comes back as a usage error.
func newFlagSet(command string, help *bool) *flag.FlagSet {
	set := flag.NewFlagSet(command, flag.ContinueOnError)
	set.SetOutput(io.Discard)
	set.BoolVar(help, "h", false, "")
	set.BoolVar(help, "help", false, "")
	return set
}

// appendTo returns the function of an option that adds each use's value to
// list.
func appendTo(list *[]string) func(string) error {
	return func(value string) error {
		*list = append(*list, value)
		return nil
	}
}

// inputFlags returns the flag set of detect or timeline with the options
// both take, reading into options.
func inputFlags(command string, options *InputOptions, help *bool) *flag.FlagSet {
	set := newFlagSet(command, help)
	set.StringVar(&options.Host, "host", "", "")
	set.StringVar(&options.User, "user", "", "")
	set.Func("work-dir", "", func(value string) error {
		options.WorkDir, options.workDirSet = value, true
		return nil
	})
	set.BoolVar(&options.KeepExtracted, "keep-extracted", false, "")
	return set
}

// detectFlags returns the flag set of detect, reading into options.
func detectFlags(options *DetectOptions, help *bool) *flag.FlagSet {
	set := inputFlags("detect", &options.InputOptions, help)
	set.BoolVar(&options.JSON, "json", false, "")
	set.BoolVar(&options.Files, "files", false, "")
	return set
}

// timelineFlags returns the flag set of timeline, reading into options.
func timelineFlags(options *TimelineOptions, help *bool) *flag.FlagSet {
	set := inputFlags("timeline", &options.InputOptions, help)
	set.Func("agent", "", appendTo(&options.Agents))
	set.StringVar(&options.Since, "since", "", "")
	set.StringVar(&options.Until, "until", "", "")
	set.Func("match", "", appendTo(&options.Match))
	set.BoolVar(&options.IncludeThinking, "include-thinking", false, "")
	set.BoolVar(&options.KeepUndated, "keep-undated", false, "")
	set.BoolVar(&options.IgnoreCase, "i", false, "")
	set.BoolVar(&options.IgnoreCase, "ignore-case", false, "")
	return set
}

// fleetFlags returns the flag set of fleet, reading into options.
func fleetFlags(options *FleetOptions, help *bool) *flag.FlagSet {
	set := newFlagSet("fleet", help)
	set.StringVar(&options.Output, "o", DefaultFleetName, "")
	set.StringVar(&options.Output, "output", DefaultFleetName, "")
	return set
}

// catalogFlags returns the flag set of catalog, reading into options.
func catalogFlags(options *CatalogOptions, help *bool) *flag.FlagSet {
	set := newFlagSet("catalog", help)
	set.BoolVar(&options.Agents, "agents", false, "")
	return set
}

// FlagSet returns a new flag set of the analysis command, holding its
// options, so a caller can tell which options take a value; nil for a
// command that is not one of Commands.
func FlagSet(command string) *flag.FlagSet {
	switch command {
	case "detect":
		return detectFlags(&DetectOptions{}, new(bool))
	case "timeline":
		return timelineFlags(&TimelineOptions{}, new(bool))
	case "fleet":
		return fleetFlags(&FleetOptions{}, new(bool))
	case "catalog":
		return catalogFlags(&CatalogOptions{}, new(bool))
	}
	return nil
}

// usageError returns the usage error Run prints for a mistake on the
// command line: the binary's and command's names and message, then the
// command's usage text.
func usageError(command, message string) error {
	return errors.New("doubleagent " + command + ": " + message + "\n" +
		strings.TrimSuffix(usageOf(command), "\n"))
}

// parse reads args into set and returns the inputs, the arguments after the
// options. An input that starts with a dash is an option placed after the
// first input, which the commands refuse, unless "--" ended the options.
func parse(set *flag.FlagSet, args []string) ([]string, error) {
	if err := set.Parse(args); err != nil {
		return nil, usageError(set.Name(), err.Error())
	}
	inputs := set.Args()
	// The options before a last "--" read again without an error only when
	// that "--" ended them: as an option's value it leaves the option short.
	options := args[:len(args)-len(inputs)]
	if last := len(options) - 1; last >= 0 && options[last] == "--" &&
		FlagSet(set.Name()).Parse(options[:last]) == nil {
		return inputs, nil
	}
	for _, input := range inputs {
		if len(input) > 1 && input[0] == '-' {
			return nil, usageError(set.Name(), "options go before the input; "+input+" is after it")
		}
	}
	return inputs, nil
}

// oneInput returns the one input of command, or the usage error of inputs
// that are not one, naming also what else the line misses.
func oneInput(command string, inputs, missing []string) (string, error) {
	if len(inputs) == 0 {
		missing = append([]string{"input"}, missing...)
	}
	switch {
	case len(missing) > 0:
		return "", usageError(command, "missing argument: "+strings.Join(missing, ", "))
	case len(inputs) > 1:
		return "", usageError(command, "unexpected argument: "+strings.Join(inputs[1:], " "))
	}
	return inputs[0], nil
}

// ParseDetect reads the arguments after `detect`. It reports whether the
// help was asked for.
func ParseDetect(args []string) (DetectOptions, bool, error) {
	var options DetectOptions
	var help bool
	inputs, err := parse(detectFlags(&options, &help), args)
	if err != nil || help {
		return options, help && err == nil, err
	}
	options.Input, err = oneInput("detect", inputs, nil)
	return options, false, err
}

// ParseTimeline reads the arguments after `timeline`: the options, the input
// and the output directory. It reports whether the help was asked for.
func ParseTimeline(args []string) (TimelineOptions, bool, error) {
	var options TimelineOptions
	var help bool
	inputs, err := parse(timelineFlags(&options, &help), args)
	if err != nil || help {
		return options, help && err == nil, err
	}
	switch {
	case len(inputs) == 0:
		return options, false, usageError("timeline", "missing argument: input, output directory")
	case len(inputs) == 1 || inputs[1] == "":
		return options, false, usageError("timeline", "missing argument: output directory")
	case len(inputs) > 2:
		return options, false, usageError("timeline", "unexpected argument: "+
			strings.Join(inputs[2:], " "))
	}
	options.Input, options.Output = inputs[0], inputs[1]
	return options, false, nil
}

// ParseFleet reads the arguments after `fleet`. It reports whether the help
// was asked for.
func ParseFleet(args []string) (FleetOptions, bool, error) {
	var options FleetOptions
	var help bool
	inputs, err := parse(fleetFlags(&options, &help), args)
	switch {
	case err != nil || help:
		return options, help && err == nil, err
	case len(inputs) == 0:
		return options, false, usageError("fleet", "missing argument: input")
	}
	options.Inputs = inputs
	return options, false, nil
}

// ParseCatalog reads the arguments after `catalog`. It reports whether the
// help was asked for.
func ParseCatalog(args []string) (CatalogOptions, bool, error) {
	var options CatalogOptions
	var help bool
	inputs, err := parse(catalogFlags(&options, &help), args)
	switch {
	case err != nil || help:
		return options, help && err == nil, err
	case len(inputs) > 0:
		return options, false, usageError("catalog", "unexpected argument: "+strings.Join(inputs, " "))
	}
	return options, false, nil
}
