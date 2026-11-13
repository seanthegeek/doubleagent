package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"no command prints the usage on stderr", nil, exitUsage, "", "Usage: doubleagent"},
		{"--version prints the version", []string{"--version"}, exitOK, "doubleagent 0.0.0-dev", ""},
		{"-V is --version", []string{"-V"}, exitOK, "doubleagent 0.0.0-dev", ""},
		{"--help prints the usage on stdout", []string{"--help"}, exitOK, "Usage: doubleagent", ""},
		{"an unknown command is a usage error", []string{"frobnicate"}, exitUsage, "", "Unknown command: frobnicate"},
		{"collect --version", []string{"collect", "--version"}, exitOK, "doubleagent 0.0.0-dev", ""},
		{"collect -h prints the collect usage", []string{"collect", "-h"}, exitOK, "Usage: doubleagent collect [options]", ""},
		{"collect --list prints the catalog", []string{"collect", "--list"}, exitOK, "# home artifacts", ""},
		{"collect with an unknown option", []string{"collect", "--bogus"}, exitUsage, "", "Unknown option: --bogus\nUsage: doubleagent collect"},
		{"collect with a bad size", []string{"collect", "--max-file-size", "x"}, exitUsage, "", "Invalid --max-file-size: x\n"},
		{"collect with a bad root", []string{"collect", "-r", "/no/such/dir"}, exitFatal, "", "Root is not a directory: /no/such/dir"},
	}
	for _, testCase := range cases {
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
