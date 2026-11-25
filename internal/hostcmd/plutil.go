package hostcmd

import (
	"context"
	"strings"
)

// Plutil reads launchd property lists through plutil, the property list tool
// macOS ships, so a value is the one launchd reads (binary plists, comments,
// CDATA sections and a repeated key's last value included). No plist is ever
// parsed by hand.
type Plutil struct {
	runner Runner
}

// NewPlutil returns a Plutil that runs plutil through runner.
func NewPlutil(runner Runner) Plutil {
	return Plutil{runner: runner}
}

// Available reports whether plutil is on the search path; it is not off macOS.
func (plutil Plutil) Available() bool {
	return plutil.runner.Available("plutil")
}

// Lint reports whether `plutil -lint` accepts the file at path.
func (plutil Plutil) Lint(ctx context.Context, path string) bool {
	_, err := plutil.runner.Output(ctx, "plutil", "-lint", path)
	return err == nil
}

// ExtractString reads the value at keyPath (`EnvironmentVariables.NAME`,
// `UserName`) of the plist at path: `plutil -type KEYPATH` names its type,
// then `plutil -extract KEYPATH raw` prints a string as written, followed by
// one newline. found is false when plutil finds no value there. Otherwise
// value holds the string, or reason says why there is none: `holds a newline`
// for a string spanning lines, `not a string` for any other type.
func (plutil Plutil) ExtractString(ctx context.Context, path, keyPath string) (value, reason string,
	found bool,
) {
	kind, err := plutil.runner.Output(ctx, "plutil", "-type", keyPath, path)
	if err != nil {
		return "", "", false
	}
	if string(kind) != "string\n" {
		return "", "not a string", true
	}
	output, err := plutil.runner.Output(ctx, "plutil", "-extract", keyPath, "raw", "-o", "-", path)
	if err != nil {
		return "", "", false
	}
	text := strings.TrimSuffix(string(output), "\n")
	if strings.Contains(text, "\n") {
		return "", "holds a newline", true
	}
	return text, "", true
}
