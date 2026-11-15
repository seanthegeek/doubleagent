//go:build parity && windows

// The Windows table runs the PowerShell collector under Windows PowerShell
// 5.1 and the binary over smoke.ps1's image and compares what they wrote
// after the normalisation in normalise_windows_test.go, which drops exactly
// the differences the milestone plans decided: the M1 exceptions (volatile
// fields, tool and version, capabilities, archive bytes, error texts) and the
// Windows ones (collection.json compared as parsed values, file-hint values,
// the start lines' shape, the log lines only one collector writes, the two
// mapped wordings, error records on stderr, the inventory log's order and
// the link targets' spelling). Every other difference fails the case.

package parity

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

// powerShell returns the path of Windows PowerShell 5.1, the oracle's host.
func powerShell() string {
	return filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
}

// longPath returns path with its links resolved and every component in its
// long form: the runner's TEMP holds an 8.3 short name, which the two
// collectors would otherwise print in different spellings.
func longPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// smokeImage builds smoke.ps1's main image without running its checks. Its
// temporary directory is the test's, so the image goes when the test ends.
func smokeImage(t *testing.T, repository string) image {
	t.Helper()
	temporary := longPath(t, t.TempDir())
	command := exec.Command(powerShell(), "-NoProfile", "-ExecutionPolicy", "Bypass", "-File",
		filepath.Join(repository, "collectors", "tests", "smoke.ps1"))
	command.Env = append(os.Environ(), "SMOKE_IMAGE_ONLY=1", "TEMP="+temporary, "TMP="+temporary)
	output, err := command.Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("smoke image: %v\n%s\n%s", err, output, exitErr.Stderr)
	} else if err != nil {
		t.Fatalf("smoke image: %v", err)
	}
	lines := nonEmptyLines(strings.ReplaceAll(string(output), "\r\n", "\n"))
	if len(lines) == 0 {
		t.Fatal("smoke image: smoke.ps1 printed no work directory")
	}
	work := longPath(t, lines[len(lines)-1])
	root := longPath(t, filepath.Join(work, "root"))
	t.Cleanup(func() { removeTree(work) })
	settleTree(t, work)
	return image{work: work, root: root, alice: root + `\Users\alice`, bob: root + `\home\bob`}
}

// executableSuffix is what a program file name ends in: go build -o keeps
// the name it is given, and Windows starts only a file with an executable
// extension.
const executableSuffix = ".exe"

// powerShellCollector returns the oracle that runs the PowerShell collector
// script in Windows PowerShell 5.1. It goes through -Command, not -File: an
// argument after -File reaches the script as one string, so two projects
// would arrive as the single value "a,b". The error record of a failed run
// is kept without its whitespace, since 5.1 wraps it at the console width,
// with the output directory replaced as execute replaces it elsewhere.
func powerShellCollector(script string) oracle {
	return func(t *testing.T, run invocation) collectorRun {
		t.Helper()
		run.args = []string{"-Command", powerShellCommand(t, script, run.args)}
		result := execute(t, []string{powerShell(), "-NoProfile", "-ExecutionPolicy", "Bypass"}, run)
		if result.code != 0 {
			replacements := []string{}
			if run.outDir != "" {
				replacements = append(replacements, withoutSpace(run.outDir), "OUT")
			}
			result.stderr = strings.NewReplacer(replacements...).Replace(withoutSpace(result.stderr))
		}
		return result
	}
}

// powerShellCommand returns the -Command text that runs the script with the
// binary's arguments mapped to the script's parameters and exits with the
// script's exit code.
func powerShellCommand(t *testing.T, script string, args []string) string {
	t.Helper()
	words := append([]string{"&", quotePowerShell(script)}, powerShellArgs(t, args)...)
	return strings.Join(words, " ") + "; exit $LASTEXITCODE"
}

// powerShellParameter returns the PowerShell collector's parameter for one
// of the binary's options and whether it takes a value, or "" for an option
// the collector has no parameter for.
func powerShellParameter(option string) (parameter string, takesValue bool) {
	switch option {
	case "-r":
		return "-Root", true
	case "-o":
		return "-OutputDir", true
	case "-u":
		return "-Users", true
	case "-p":
		return "-Project", true
	case "--max-file-size":
		return "-MaxFileSizeMB", true
	case "--env-scan-max-size":
		return "-EnvScanMaxSizeMB", true
	}
	switches := map[string]string{"-q": "-Quiet", "--no-docker": "-NoDocker",
		"--no-projects": "-NoProjects", "--no-secrets": "-NoSecrets", "--full": "-Full",
		"--env-scan-home": "-EnvScanHome", "--inventory": "-Inventory", "-k": "-KeepStaging"}
	return switches[option], false
}

// powerShellArgs maps the binary's options one to one onto the collector's
// parameters, each value a quoted literal, and every -p value into one
// -Project list.
func powerShellArgs(t *testing.T, args []string) []string {
	t.Helper()
	var words, projects []string
	for i := 0; i < len(args); i++ {
		parameter, takesValue := powerShellParameter(args[i])
		switch {
		case parameter == "" || (takesValue && i+1 == len(args)):
			t.Fatalf("no PowerShell parameter for the option %q in %q", args[i], args)
		case !takesValue:
			words = append(words, parameter)
		case args[i] == "-p":
			i++
			projects = append(projects, quotePowerShell(args[i]))
		default:
			i++
			words = append(words, parameter, quotePowerShell(args[i]))
		}
	}
	if len(projects) != 0 {
		words = append(words, "-Project", strings.Join(projects, ","))
	}
	return words
}

// quotePowerShell returns text as a single-quoted PowerShell literal, in
// which nothing is expanded and a quote is written twice.
func quotePowerShell(text string) string {
	return "'" + strings.ReplaceAll(text, "'", "''") + "'"
}

// withoutSpace returns text with every white space character removed.
func withoutSpace(text string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsSpace(character) {
			return -1
		}
		return character
	}, text)
}

func TestCollectAgreesWithThePowerShellCollector(t *testing.T) {
	repository := repositoryRoot(t)
	binary := []string{buildBinary(t, repository), "collect"}
	img := smokeImage(t, repository)
	reference := powerShellCollector(filepath.Join(repository, "collectors", "Collect-AgentArtifacts.ps1"))
	cases := windowsCases()
	t.Run("shared image", func(t *testing.T) {
		for _, testCase := range cases {
			if testCase.parallel {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()
					testCase.run(t, img, reference, binary)
				})
			}
		}
	})
	for _, testCase := range cases {
		if !testCase.parallel {
			t.Run(testCase.name, func(t *testing.T) { testCase.run(t, img, reference, binary) })
		}
	}
}
