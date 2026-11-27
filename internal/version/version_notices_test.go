package version

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// repositoryRoot is the checkout's root, relative to this package's directory,
// where go test runs.
const repositoryRoot = "../.."

// noticeRule is the line above and below each "File:" line of the notices.
const noticeRule = "--------------------------------------------------------------------------------"

// readText returns the file's text with CRLF line ends read as LF, since a
// Windows checkout may convert the text files' line ends.
func readText(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(content), "\r\n", "\n")
}

// noticesText returns THIRD_PARTY_NOTICES.txt, the licences the release ships
// beside the binaries.
func noticesText(t *testing.T) string {
	t.Helper()
	return readText(t, filepath.Join(repositoryRoot, "THIRD_PARTY_NOTICES.txt"))
}

// vendoredModules returns "<path> <version>" for each module that
// vendor/modules.txt lists, the modules the binary can link; the go command
// refuses a build whose go.mod and modules.txt disagree.
func vendoredModules(t *testing.T) []string {
	t.Helper()
	var modules []string
	scanner := bufio.NewScanner(strings.NewReader(readText(t,
		filepath.Join(repositoryRoot, "vendor", "modules.txt"))))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 3 && fields[0] == "#" {
			modules = append(modules, fields[1]+" "+fields[2])
		}
	}
	return modules
}

// noticeBlock returns the header and text of one licence file as the notices
// hold it.
func noticeBlock(name, text string) string {
	return noticeRule + "\nFile: " + name + "\n" + noticeRule + "\n" + text
}

// isLicenceFile reports whether a file name is one a module uses for its
// licence, patent grant or notices.
func isLicenceFile(name string) bool {
	upper := strings.ToUpper(name)
	for _, prefix := range []string{"LICENSE", "LICENCE", "COPYING", "NOTICE", "PATENTS"} {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

func TestTheNoticesNameEveryVendoredModuleAtItsVersionAndNoOther(t *testing.T) {
	t.Parallel()
	var listed []string
	for line := range strings.SplitSeq(noticesText(t), "\n") {
		if module, ok := strings.CutPrefix(line, "Module: "); ok {
			listed = append(listed, module)
		}
	}
	vendored := vendoredModules(t)
	for _, module := range vendored {
		if !slices.Contains(listed, module) {
			t.Errorf("THIRD_PARTY_NOTICES.txt has no \"Module: %s\" section", module)
		}
	}
	for _, module := range listed {
		if !slices.Contains(vendored, module) {
			t.Errorf("THIRD_PARTY_NOTICES.txt names %s, which vendor/modules.txt does not list",
				module)
		}
	}
}

func TestTheNoticesHoldEveryLicenceFileOfTheVendoredModulesVerbatim(t *testing.T) {
	t.Parallel()
	notices := noticesText(t)
	found := 0
	for _, module := range vendoredModules(t) {
		modulePath, _, _ := strings.Cut(module, " ")
		moduleDirectory := filepath.Join(repositoryRoot, "vendor", filepath.FromSlash(modulePath))
		err := filepath.WalkDir(moduleDirectory, func(path string, entry fs.DirEntry,
			err error) error {
			if err != nil || entry.IsDir() || !isLicenceFile(entry.Name()) {
				return err
			}
			relative, err := filepath.Rel(repositoryRoot, path)
			if err != nil {
				return err
			}
			found++
			name := filepath.ToSlash(relative)
			if !strings.Contains(notices, noticeBlock(name, readText(t, path))) {
				t.Errorf("THIRD_PARTY_NOTICES.txt does not hold %s verbatim under its File: line",
					name)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if found == 0 {
		t.Fatal("no licence file found under vendor/")
	}
}

// toolchainVersion returns the toolchain version a module's go and toolchain
// lines pin, without the "go" prefix: the toolchain line's, which the go
// command prefers to the go line's, unless it is absent or "default", which
// names no version and leaves the go line's.
func toolchainVersion(goLine, toolchainLine string) string {
	if version, ok := strings.CutPrefix(toolchainLine, "go"); ok {
		return version
	}
	return goLine
}

// pinnedToolchain returns the version of the Go toolchain go.mod pins, as the
// go command reads the file (go mod edit -json): its toolchain line when it
// has one, else its go line (toolchainVersion).
func pinnedToolchain(t *testing.T) string {
	t.Helper()
	command := exec.Command("go", "mod", "edit", "-json")
	command.Dir = repositoryRoot
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go mod edit -json: %v", err)
	}
	var module struct{ Go, Toolchain string }
	if err := json.Unmarshal(output, &module); err != nil {
		t.Fatalf("go mod edit -json: %v", err)
	}
	return toolchainVersion(module.Go, module.Toolchain)
}

func TestTheNoticesHoldTheGoDistributionsLicenceAtTheVersionGoModNames(t *testing.T) {
	t.Parallel()
	notices := noticesText(t)
	goVersion := pinnedToolchain(t)
	toolchain := "Toolchain: go" + goVersion + " (the Go runtime and standard library)"
	if goVersion == "" || !strings.Contains(notices, toolchain+"\n") {
		t.Errorf("THIRD_PARTY_NOTICES.txt has no %q line", toolchain)
	}
	output, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatalf("go env GOROOT: %v", err)
	}
	goRoot := strings.TrimSpace(string(output))
	for _, name := range []string{"LICENSE", "PATENTS"} {
		block := noticeBlock(name+" (of the Go distribution)",
			readText(t, filepath.Join(goRoot, name)))
		if !strings.Contains(notices, block) {
			t.Errorf("THIRD_PARTY_NOTICES.txt does not hold the Go distribution's %s verbatim",
				name)
		}
	}
}

func TestTheToolchainLineWinsOverTheGoLine(t *testing.T) {
	cases := []struct {
		name, goLine, toolchainLine, want string
	}{
		{"a module without a toolchain line uses its go line", "1.27.4", "", "1.27.4"},
		{"a toolchain line names the toolchain", "1.27.4", "go1.27.5", "1.27.5"},
		{"the default toolchain leaves the go line", "1.27.4", "default", "1.27.4"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := toolchainVersion(testCase.goLine, testCase.toolchainLine); got != testCase.want {
				t.Errorf("toolchainVersion(%q, %q) = %q, want %q", testCase.goLine,
					testCase.toolchainLine, got, testCase.want)
			}
		})
	}
}
