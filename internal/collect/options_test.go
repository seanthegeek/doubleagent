package collect

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
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

// rootCase is a -r value and the root ResolveRoot should make of it, or
// the error it should give.
type rootCase struct {
	name      string
	root      string
	want      string
	wantError string
}

func TestResolveRoot(t *testing.T) {
	native := longTempDir(t)
	if err := os.MkdirAll(filepath.Join(native, "img", "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(native, "file"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	base := syspath.FromNative(native)
	getwd := func() (string, error) { return base, nil }
	cases := append([]rootCase{
		{"no root is live mode", "", "", ""},
		{"a relative root becomes absolute and canonical", "img/etc/..", base + "/img", ""},
		{"an absolute root is kept", base + "/img", base + "/img", ""},
		{"an absolute native root is made canonical", filepath.Join(native, "img"), base + "/img", ""},
		{"a file is not a root", base + "/file", "", "Root is not a directory: " + base + "/file"},
		{"a missing root is not a root", "nope", "", "Root is not a directory: nope"},
	}, platformRootCases(base)...)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ResolveRoot(Options{Root: testCase.root}, getwd)
			if testCase.wantError != "" {
				if err == nil || err.Error() != testCase.wantError {
					t.Fatalf("error %v, want %q", err, testCase.wantError)
				}
				return
			}
			if err != nil || got.Root != testCase.want {
				t.Errorf("root %q, error %v; want %q", got.Root, err, testCase.want)
			}
		})
	}
}

func TestRespellProjectsMovesTheTypedRootToTheResolvedOne(t *testing.T) {
	typed, resolved := "/t/JOHNSM~1/img", "/t/johnsmith/img"
	got := respellProjects([]string{typed + "/proj", typed, typed + "2/proj", "/t/JOHNSM~1/other"},
		typed, resolved)
	want := []string{resolved + "/proj", resolved, typed + "2/proj", "/t/JOHNSM~1/other"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("respellProjects = %q, want %q", got, want)
	}
}
