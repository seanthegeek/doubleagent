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
		"-k", "--inventory", "-q", "--max-file-size", "0", "--env-scan-max-size", "3",
		"--no-preserve-atime"}
	options, action, err := ParseOptions(args)
	if err != nil || action != ActRun {
		t.Fatalf("action %v, error %v", action, err)
	}
	want := Options{OutDir: "out", OutDirSet: true, Root: "img", UsersFilter: "a,b",
		Projects: []string{"x", "y"}, Full: true, NoSecrets: true, NoProjects: true,
		NoDocker: true, EnvScanHome: true, KeepStaging: true, Inventory: true, Quiet: true,
		MaxFileBytes: 0, EnvScanMaxBytes: 3 * mebibyte, NoPreserveAtime: true}
	if !reflect.DeepEqual(options, want) {
		t.Errorf("options %+v, want %+v", options, want)
	}
}

// defaultOptions returns the options of an empty command line with change
// applied.
func defaultOptions(change func(*Options)) Options {
	options := Options{OutDir: ".", MaxFileBytes: 256 * mebibyte, EnvScanMaxBytes: mebibyte}
	change(&options)
	return options
}

func TestEveryOptionIsReadInEachSpelling(t *testing.T) {
	setOut := func(options *Options) { options.OutDir, options.OutDirSet = "out", true }
	cases := []struct {
		name string
		args []string
		want Options
	}{
		{"-o takes the next argument", []string{"-o", "out"}, defaultOptions(setOut)},
		{"--output takes the next argument", []string{"--output", "out"}, defaultOptions(setOut)},
		{"--output takes a value after =", []string{"--output=out"}, defaultOptions(setOut)},
		{"-output with one dash is --output", []string{"-output", "out"}, defaultOptions(setOut)},
		{"-o takes a value after =", []string{"-o=out"}, defaultOptions(setOut)},
		{"an empty output is kept as given", []string{"-o", ""},
			defaultOptions(func(options *Options) { options.OutDir, options.OutDirSet = "", true })},
		{"-r and --root name the root", []string{"-r", "a", "--root", "img"},
			defaultOptions(func(options *Options) { options.Root = "img" })},
		{"-u and --users name the users", []string{"--users", "a,b"},
			defaultOptions(func(options *Options) { options.UsersFilter = "a,b" })},
		{"-u is --users", []string{"-u", "bob"},
			defaultOptions(func(options *Options) { options.UsersFilter = "bob" })},
		{"-p and --project repeat in order", []string{"-p", "x", "--project", "y", "-p=z", "--project=x"},
			defaultOptions(func(options *Options) { options.Projects = []string{"x", "y", "z", "x"} })},
		{"-k and --keep-staging keep the staging", []string{"--keep-staging"},
			defaultOptions(func(options *Options) { options.KeepStaging = true })},
		{"-k is --keep-staging", []string{"-k"},
			defaultOptions(func(options *Options) { options.KeepStaging = true })},
		{"-q and --quiet are one switch", []string{"--quiet"},
			defaultOptions(func(options *Options) { options.Quiet = true })},
		{"a switch takes =false", []string{"-q", "--quiet=false"}, defaultOptions(func(*Options) {})},
		{"a switch takes =true", []string{"--full=true", "--no-docker=1"},
			defaultOptions(func(options *Options) { options.Full, options.NoDocker = true, true })},
		{"the sizes take a value after =", []string{"--max-file-size=2", "-env-scan-max-size", "0"},
			defaultOptions(func(options *Options) { options.MaxFileBytes, options.EnvScanMaxBytes = 2*mebibyte, 0 })},
		{"the last size given counts", []string{"--max-file-size", "x", "--max-file-size", "4"},
			defaultOptions(func(options *Options) { options.MaxFileBytes = 4 * mebibyte })},
		{"a value may start with a dash", []string{"-o", "-q"},
			defaultOptions(func(options *Options) { options.OutDir, options.OutDirSet = "-q", true })},
		{"-- ends the options", []string{"-q", "--"},
			defaultOptions(func(options *Options) { options.Quiet = true })},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			options, action, err := ParseOptions(test.args)
			if err != nil || action != ActRun {
				t.Fatalf("action %v, error %v", action, err)
			}
			if !reflect.DeepEqual(options, test.want) {
				t.Errorf("options %+v, want %+v", options, test.want)
			}
		})
	}
}

func TestNoPreserveAtimeIsASwitch(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"it is off by default", nil, false},
		{"it is set on its own", []string{"--no-preserve-atime"}, true},
		{"the argument after it is still an option", []string{"--no-preserve-atime", "-q"}, true},
		{"it takes =1", []string{"--no-preserve-atime=1"}, true},
		{"it takes =false", []string{"--no-preserve-atime", "--no-preserve-atime=false"}, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			options, action, err := ParseOptions(test.args)
			if err != nil || action != ActRun || options.NoPreserveAtime != test.want {
				t.Errorf("NoPreserveAtime %v, action %v, error %v; want %v",
					options.NoPreserveAtime, action, err, test.want)
			}
		})
	}
}

func TestUsageTextNamesNoPreserveAtime(t *testing.T) {
	want := "      --no-preserve-atime Open files read-only; reading may change their\n" +
		"                          access time on disk\n"
	if !strings.Contains(UsageText, want) {
		t.Errorf("usage text lacks %q:\n%s", want, UsageText)
	}
}

func TestParseOptionsActions(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want Action
	}{
		{"--version", []string{"--version"}, ActVersion},
		{"-V", []string{"-V"}, ActVersion},
		{"-version with one dash", []string{"-version"}, ActVersion},
		{"--help", []string{"--help"}, ActHelp},
		{"-h", []string{"-h"}, ActHelp},
		{"-help with one dash", []string{"-help"}, ActHelp},
		{"--list", []string{"--list"}, ActList},
		{"--version after other options", []string{"-o", "out", "-q", "--version"}, ActVersion},
		{"--help wins over --version wherever it stands", []string{"-V", "-h"}, ActHelp},
		{"--version wins over --list", []string{"--list", "-V"}, ActVersion},
		{"--list skips the size check", []string{"--list", "--max-file-size", "x"}, ActList},
		{"--version skips the size check", []string{"--max-file-size", "x", "-V"}, ActVersion},
		{"--list=false runs", []string{"--list", "--list=false"}, ActRun},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, action, err := ParseOptions(test.args)
			if err != nil || action != test.want {
				t.Errorf("action %v, error %v; want %v", action, err, test.want)
			}
		})
	}
}

func TestParseOptionsErrors(t *testing.T) {
	usage := UsageText
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"an unknown option", []string{"--bogus"},
			"doubleagent collect: flag provided but not defined: -bogus\n" + usage},
		{"an unknown option before --version", []string{"--bogus", "-V"},
			"doubleagent collect: flag provided but not defined: -bogus\n" + usage},
		{"an unknown option after --version", []string{"-V", "--bogus"},
			"doubleagent collect: flag provided but not defined: -bogus\n" + usage},
		{"an unknown option after --help", []string{"-h", "--bogus"},
			"doubleagent collect: flag provided but not defined: -bogus\n" + usage},
		{"an unknown option after --list", []string{"--list", "--bogus"},
			"doubleagent collect: flag provided but not defined: -bogus\n" + usage},
		{"a short option with its value attached", []string{"-oout"},
			"doubleagent collect: flag provided but not defined: -oout\n" + usage},
		{"a missing value", []string{"-o"}, "doubleagent collect: flag needs an argument: -o\n" + usage},
		{"a missing size value", []string{"--max-file-size"},
			"doubleagent collect: flag needs an argument: -max-file-size\n" + usage},
		{"a switch with a value that is not a boolean", []string{"-q=x"},
			"doubleagent collect: invalid boolean value \"x\" for -q: parse error\n" + usage},
		{"a stray argument", []string{"-q", "stray"}, "doubleagent collect: unexpected argument: stray\n" + usage},
		{"an argument after --", []string{"--", "-q"}, "doubleagent collect: unexpected argument: -q\n" + usage},
		{"a lone dash", []string{"-"}, "doubleagent collect: unexpected argument: -\n" + usage},
		{"a stray argument before --version", []string{"stray", "-V"},
			"doubleagent collect: unexpected argument: stray\n" + usage},
		{"a non-numeric max size", []string{"--max-file-size", "12x"}, "Invalid --max-file-size: 12x\n"},
		{"an empty max size", []string{"--max-file-size", ""}, "Invalid --max-file-size: \n"},
		{"a negative scan size", []string{"--env-scan-max-size", "-1"}, "Invalid --env-scan-max-size: -1\n"},
		{"a size that overflows", []string{"--max-file-size", "99999999999999999999"},
			"Invalid --max-file-size: 99999999999999999999\n"},
		{"the max size is checked first", []string{"--env-scan-max-size", "q", "--max-file-size", "z"},
			"Invalid --max-file-size: z\n"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, action, err := ParseOptions(test.args)
			var usageError *UsageError
			if !errors.As(err, &usageError) || action != ActRun {
				t.Fatalf("action %v, error %v; want a *UsageError", action, err)
			}
			var printed bytes.Buffer
			usageError.Print(&printed)
			if printed.String() != test.want {
				t.Errorf("printed %q, want %q", printed.String(), test.want)
			}
		})
	}
}

func TestAUsageErrorPrintsTheWholeUsageOrNone(t *testing.T) {
	var printed bytes.Buffer
	(&UsageError{Message: "doubleagent collect: x", ShowUsage: true}).Print(&printed)
	if printed.String() != "doubleagent collect: x\n"+UsageText {
		t.Errorf("printed %q", printed.String())
	}
	printed.Reset()
	(&UsageError{Message: "Invalid --max-file-size: z"}).Print(&printed)
	if printed.String() != "Invalid --max-file-size: z\n" {
		t.Errorf("printed %q", printed.String())
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
