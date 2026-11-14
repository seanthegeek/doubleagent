package collect

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseOptionsDefaults(t *testing.T) {
	options, action, err := ParseOptions(nil)
	if err != nil || action != ActRun {
		t.Fatalf("action %v, error %v", action, err)
	}
	want := Options{OutDir: ".", MaxFileBytes: 256 * mebibyte, EnvScanMaxBytes: mebibyte}
	if !reflect.DeepEqual(options, want) {
		t.Errorf("options %+v, want %+v", options, want)
	}
}

func TestParseOptionsReadsEveryOption(t *testing.T) {
	args := []string{"-o", "out", "-r", "img", "-u", "a,b", "-p", "x", "--project", "y",
		"--full", "--no-secrets", "--no-projects", "--no-docker", "--env-scan-home",
		"-k", "--inventory", "-q", "--max-file-size", "0", "--env-scan-max-size", "3"}
	options, action, err := ParseOptions(args)
	if err != nil || action != ActRun {
		t.Fatalf("action %v, error %v", action, err)
	}
	want := Options{OutDir: "out", OutDirSet: true, Root: "img", UsersFilter: "a,b",
		Projects: []string{"x", "y"}, Full: true, NoSecrets: true, NoProjects: true,
		NoDocker: true, EnvScanHome: true, KeepStaging: true, Inventory: true, Quiet: true,
		MaxFileBytes: 0, EnvScanMaxBytes: 3 * mebibyte}
	if !reflect.DeepEqual(options, want) {
		t.Errorf("options %+v, want %+v", options, want)
	}
}

func TestParseOptionsActionsAndErrors(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantAction Action
		wantStderr string
	}{
		{"--version", []string{"--version"}, ActVersion, ""},
		{"-V stops before a later bad option", []string{"-V", "--bogus"}, ActVersion, ""},
		{"-h", []string{"-h"}, ActHelp, ""},
		{"--list", []string{"--list"}, ActList, ""},
		{"--list skips the size check", []string{"--list", "--max-file-size", "x"}, ActList, ""},
		{"an earlier bad option beats --version", []string{"--bogus", "-V"}, ActRun,
			"Unknown option: --bogus\nUsage: doubleagent collect [options]\n"},
		{"a later bad option beats --list", []string{"--list", "--bogus"}, ActRun,
			"Unknown option: --bogus\n"},
		{"a missing value shows only the usage", []string{"-o"}, ActRun,
			"Usage: doubleagent collect [options]\n"},
		{"a missing size value", []string{"--max-file-size"}, ActRun, "Usage: doubleagent"},
		{"a non-numeric max size", []string{"--max-file-size", "12x"}, ActRun,
			"Invalid --max-file-size: 12x\n"},
		{"an empty max size", []string{"--max-file-size", ""}, ActRun,
			"Invalid --max-file-size: \n"},
		{"a negative scan size", []string{"--env-scan-max-size", "-1"}, ActRun,
			"Invalid --env-scan-max-size: -1\n"},
		{"a size that overflows", []string{"--max-file-size", "99999999999999999999"}, ActRun,
			"Invalid --max-file-size: 99999999999999999999\n"},
		{"the max size is checked first", []string{"--env-scan-max-size", "q", "--max-file-size", "z"},
			ActRun, "Invalid --max-file-size: z\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, action, err := ParseOptions(testCase.args)
			if testCase.wantStderr == "" {
				if err != nil || action != testCase.wantAction {
					t.Fatalf("action %v, error %v", action, err)
				}
				return
			}
			var usageError *UsageError
			if !errors.As(err, &usageError) {
				t.Fatalf("error %v, want a *UsageError", err)
			}
			var printed bytes.Buffer
			usageError.Print(&printed)
			if !strings.HasPrefix(printed.String(), testCase.wantStderr) {
				t.Errorf("printed %q, want prefix %q", printed.String(), testCase.wantStderr)
			}
		})
	}
}
