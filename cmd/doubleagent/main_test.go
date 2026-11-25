package main

import (
	"bytes"
	"strings"
	"testing"
)

// runCase is one command line with the exit code it gives and a text its
// stdout and its stderr each hold.
type runCase struct {
	name       string
	args       []string
	wantCode   int
	wantStdout string
	wantStderr string
}

// runCases returns the command lines TestRun runs.
func runCases() []runCase {
	return []runCase{
		{"no command prints the usage on stderr", nil, exitUsage, "", "Usage: doubleagent"},
		{"--version prints the version", []string{"--version"}, exitOK, "doubleagent 0.0.0-dev", ""},
		{"-V is --version", []string{"-V"}, exitOK, "doubleagent 0.0.0-dev", ""},
		{"--help prints the usage on stdout", []string{"--help"}, exitOK, "Usage: doubleagent", ""},
		{"help prints the usage on stdout", []string{"help"}, exitOK, "Usage: doubleagent", ""},
		{"the usage names help", []string{"--help"}, exitOK, "\n  help        print this text\n", ""},
		{"an unknown command is a usage error", []string{"frobnicate"}, exitUsage, "", "Unknown command: frobnicate"},
		{"an unknown command is printed with its control characters shown", []string{"frob\x1b[2Jnicate"},
			exitUsage, "", "Unknown command: frob\\x1b[2Jnicate"},
		{"collect --version", []string{"collect", "--version"}, exitOK, "doubleagent 0.0.0-dev", ""},
		{"collect -h prints the collect usage", []string{"collect", "-h"}, exitOK, "Usage: doubleagent collect [options]", ""},
		{"collect --list prints the catalog", []string{"collect", "--list"}, exitOK, "# home artifacts", ""},
		{"collect with an unknown option", []string{"collect", "--bogus"}, exitUsage, "",
			"doubleagent collect: flag provided but not defined: -bogus\nUsage: doubleagent collect"},
		{"collect with a missing value", []string{"collect", "-o"}, exitUsage, "",
			"doubleagent collect: flag needs an argument: -o\nUsage: doubleagent collect"},
		{"collect with a stray argument", []string{"collect", "stray"}, exitUsage, "",
			"doubleagent collect: unexpected argument: stray\nUsage: doubleagent collect"},
		{"collect --version after --output=", []string{"collect", "--output=x", "--version"}, exitOK,
			"doubleagent 0.0.0-dev", ""},
		{"collect --help after a single-dash long option", []string{"collect", "-output", "x", "--help"}, exitOK,
			"Usage: doubleagent collect [options]", ""},
		{"collect with a bad size", []string{"collect", "--max-file-size", "x"}, exitUsage, "", "Invalid --max-file-size: x\n"},
		{"collect with a bad root", []string{"collect", "-r", "/no/such/dir"}, exitFatal, "", "Root is not a directory: /no/such/dir"},
		{"catalog prints the catalog", []string{"catalog"}, exitOK, "# home artifacts", ""},
		{"catalog --agents names the agents", []string{"catalog", "--agents"}, exitOK, "claude-code ", ""},
		{"an analysis command's mistake exits 2", []string{"catalog", "--bogus"}, exitFatal, "",
			"doubleagent catalog: flag provided but not defined: -bogus\nUsage: doubleagent catalog"},
		{"a missing value exits 2", []string{"timeline", "--since"}, exitFatal, "",
			"doubleagent timeline: flag needs an argument: -since\nUsage: doubleagent timeline"},
		{"a missing output directory exits 2", []string{"timeline", "in"}, exitFatal, "",
			"doubleagent timeline: missing argument: output directory\nUsage: doubleagent timeline [options] INPUT OUTPUT_DIR"},
		{"-o is not a timeline option", []string{"timeline", "-o", "out", "in"}, exitFatal, "",
			"doubleagent timeline: flag provided but not defined: -o\nUsage: doubleagent timeline"},
		{"a missing input exits 2", []string{"detect"}, exitFatal, "",
			"doubleagent detect: missing argument: input\nUsage: doubleagent detect"},
		{"a switch value that is not a boolean exits 2", []string{"fleet", "-h=x"}, exitFatal, "",
			"doubleagent fleet: invalid boolean value \"x\" for -h: parse error\nUsage: doubleagent fleet"},
		{"an analysis command's help exits 0", []string{"timeline", "--help", "in"}, exitOK,
			"Usage: doubleagent timeline", ""},
		{"an option after the input exits 2", []string{"timeline", "in", "out", "--since", "x"}, exitFatal, "",
			"doubleagent timeline: options go before the input; --since is after it\nUsage: doubleagent timeline"},
	}
}

func TestRun(t *testing.T) {
	for _, testCase := range runCases() {
		t.Run(testCase.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(testCase.args, &stdout, &stderr)
			if code != testCase.wantCode {
				t.Errorf("exit code %d, want %d", code, testCase.wantCode)
			}
			if !strings.Contains(stdout.String(), testCase.wantStdout) {
				t.Errorf("stdout %q does not contain %q", stdout.String(), testCase.wantStdout)
			}
			if !strings.Contains(stderr.String(), testCase.wantStderr) {
				t.Errorf("stderr %q does not contain %q", stderr.String(), testCase.wantStderr)
			}
		})
	}
}
