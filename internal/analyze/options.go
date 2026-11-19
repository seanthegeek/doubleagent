package analyze

import (
	"errors"
	"flag"
	"io"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// InputOptions are the options detect and timeline share: the input and how
// it is opened.
type InputOptions struct {
	Input         string
	Host          string
	User          string
	WorkDir       string
	KeepExtracted bool
	// Secrets is how the output shows the secrets found in the evidence:
	// as written, or replaced by a placeholder under --no-secrets.
	Secrets secrets.Setting
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
		if value == "" {
			return errors.New("an empty directory name")
		}
		options.WorkDir = value
		return nil
	})
	set.BoolVar(&options.KeepExtracted, "keep-extracted", false, "")
	set.BoolVar(&options.Secrets.Withhold, "no-secrets", false, "")
	set.StringVar(&options.Secrets.Placeholder, placeholderOption, "", "")
	return set
}

// placeholderOption is the option that names the text a withheld secret is
// replaced by.
const placeholderOption = "redaction-placeholder"

// completeSecrets finishes the secrets setting of a parsed command line:
// the default placeholder when --no-secrets is given without
// --redaction-placeholder, and the usage error of --redaction-placeholder
// without --no-secrets, which would otherwise do nothing and suggest that
// secrets were withheld.
func completeSecrets(set *flag.FlagSet, options *InputOptions) error {
	given := placeholderGiven(set)
	switch {
	case given && !options.Secrets.Withhold:
		return usageError(set.Name(), "--"+placeholderOption+" needs --no-secrets")
	case !given && options.Secrets.Withhold:
		options.Secrets.Placeholder = secrets.DefaultPlaceholder
	}
	return nil
}

// placeholderGiven reports whether a parsed command line gave
// --redaction-placeholder.
func placeholderGiven(set *flag.FlagSet) bool {
	given := false
	set.Visit(func(option *flag.Flag) { given = given || option.Name == placeholderOption })
	return given
}

// argumentError returns the usage error of a mistake in the arguments of
// detect or timeline once their options are read, with the message
// withheld as the run withholds secrets, since an argument the analyst
// typed can hold a URL's credentials.
func argumentError(set *flag.FlagSet, options InputOptions, message string) error {
	setting := options.Secrets
	if setting.Withhold && !placeholderGiven(set) {
		setting.Placeholder = secrets.DefaultPlaceholder
	}
	return usageError(set.Name(), secrets.NewRecorder(setting).Text(message))
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
	return errors.New("doubleagent " + command + ": " + shown(message) + "\n" +
		strings.TrimSuffix(usageOf(command), "\n"))
}

// parse reads args into set and returns the inputs, the arguments after the
// options, or the usage error of the option parser. An input that starts
// with a dash is an option placed after the first input, which the
// commands refuse, unless "--" ended the options: misplaced is then the
// message that says so, which the command gives as its own usage error.
func parse(set *flag.FlagSet, args []string) (inputs []string, misplaced string, err error) {
	if err := set.Parse(args); err != nil {
		return nil, "", usageError(set.Name(), err.Error())
	}
	inputs = set.Args()
	// The options before a last "--" read again without an error only when
	// that "--" ended them: as an option's value it leaves the option short.
	options := args[:len(args)-len(inputs)]
	if last := len(options) - 1; last >= 0 && options[last] == "--" &&
		FlagSet(set.Name()).Parse(options[:last]) == nil {
		return inputs, "", nil
	}
	for _, input := range inputs {
		if len(input) > 1 && input[0] == '-' {
			return nil, "options go before the input; " + input + " is after it", nil
		}
	}
	return inputs, "", nil
}

// emptyInput is the usage error of an input given as empty text, which
// would otherwise name the current directory.
const emptyInput = "the input is empty; name a file or directory"

// emptyOutput is the usage error of fleet's -o given as empty text, which
// names no file.
const emptyOutput = "the output is empty; name a file"

// oneInput returns the one input of a command, or the message of the
// usage error of inputs that are not one.
func oneInput(inputs []string) (string, string) {
	switch {
	case len(inputs) == 0:
		return "", "missing argument: input"
	case len(inputs) > 1:
		return "", "unexpected argument: " + strings.Join(inputs[1:], " ")
	case inputs[0] == "":
		return "", emptyInput
	}
	return inputs[0], ""
}

// ParseDetect reads the arguments after `detect`. It reports whether the
// help was asked for.
func ParseDetect(args []string) (DetectOptions, bool, error) {
	var options DetectOptions
	var help bool
	set := detectFlags(&options, &help)
	inputs, misplaced, err := parse(set, args)
	switch {
	case err != nil:
		return options, false, err
	case misplaced != "":
		return options, false, argumentError(set, options.InputOptions, misplaced)
	case help:
		return options, true, nil
	}
	if err := completeSecrets(set, &options.InputOptions); err != nil {
		return options, false, err
	}
	input, message := oneInput(inputs)
	if message != "" {
		return options, false, argumentError(set, options.InputOptions, message)
	}
	options.Input = input
	return options, false, nil
}

// ParseTimeline reads the arguments after `timeline`: the options, the input
// and the output directory. It reports whether the help was asked for.
func ParseTimeline(args []string) (TimelineOptions, bool, error) {
	var options TimelineOptions
	var help bool
	set := timelineFlags(&options, &help)
	inputs, misplaced, err := parse(set, args)
	switch {
	case err != nil:
		return options, false, err
	case misplaced != "":
		return options, false, argumentError(set, options.InputOptions, misplaced)
	case help:
		return options, true, nil
	}
	if err := completeSecrets(set, &options.InputOptions); err != nil {
		return options, false, err
	}
	message := timelineArguments(inputs)
	if message == "" {
		message = outputNameRefusal(inputs[1])
	}
	if message != "" {
		return options, false, argumentError(set, options.InputOptions, message)
	}
	options.Input, options.Output = inputs[0], inputs[1]
	return options, false, nil
}

// timelineArguments returns the message of the usage error of timeline's
// arguments when they are not an input and an output directory, or "".
func timelineArguments(inputs []string) string {
	switch {
	case len(inputs) == 0:
		return "missing argument: input, output directory"
	case inputs[0] == "":
		return emptyInput
	case len(inputs) == 1 || inputs[1] == "":
		return "missing argument: output directory"
	case len(inputs) > 2:
		return "unexpected argument: " + strings.Join(inputs[2:], " ")
	}
	return ""
}

// ParseFleet reads the arguments after `fleet`. It reports whether the help
// was asked for.
func ParseFleet(args []string) (FleetOptions, bool, error) {
	var options FleetOptions
	var help bool
	inputs, misplaced, err := parse(fleetFlags(&options, &help), args)
	switch {
	case err != nil:
		return options, false, err
	case misplaced != "":
		return options, false, usageError("fleet", misplaced)
	case help:
		return options, true, nil
	case len(inputs) == 0:
		return options, false, usageError("fleet", "missing argument: input")
	case slices.Contains(inputs, ""):
		return options, false, usageError("fleet", emptyInput)
	case options.Output == "":
		return options, false, usageError("fleet", emptyOutput)
	case outputNameRefusal(options.Output) != "":
		return options, false, usageError("fleet", outputNameRefusal(options.Output))
	}
	options.Inputs = inputs
	return options, false, nil
}

// ParseCatalog reads the arguments after `catalog`. It reports whether the
// help was asked for.
func ParseCatalog(args []string) (CatalogOptions, bool, error) {
	var options CatalogOptions
	var help bool
	inputs, misplaced, err := parse(catalogFlags(&options, &help), args)
	switch {
	case err != nil:
		return options, false, err
	case misplaced != "":
		return options, false, usageError("catalog", misplaced)
	case help:
		return options, true, nil
	case len(inputs) > 0:
		return options, false, usageError("catalog", "unexpected argument: "+strings.Join(inputs, " "))
	}
	return options, false, nil
}
