package collect_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/collect/imagetest"
)

// collectorRun is what one run of the binary left: its exit code, its
// standard streams, the text of each output file keyed by its suffix after
// the run name, and the archive's members in archive order. outDir and name
// are the run's output directory and run name, "" when it wrote none.
type collectorRun struct {
	code    int
	stdout  string
	stderr  string
	files   map[string]string
	members []archiveMember
	outDir  string
	name    string
}

// archiveMember is one member of a run's archive: its name and a
// description of its content, the SHA-256 of a regular file, the target of
// a link or the type of anything else. text holds a regular file's content
// when it copies a file written beside the archive (archivedCopies).
type archiveMember struct {
	name        string
	description string
	text        string
}

// invocation is one run of the binary: its arguments after "collect", its
// working directory and extra environment, and the directory its output
// files land in ("" for a run that writes none).
type invocation struct {
	args   []string
	cwd    string
	env    []string
	outDir string
}

// repositoryRoot returns the repository's absolute path, two levels above
// this package.
func repositoryRoot(test *testing.T) string {
	test.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		test.Fatal(err)
	}
	return root
}

// buildBinary builds the doubleagent command into a temporary directory and
// returns its path. The tests run it as a subprocess, built without the race
// detector, so a run under -race takes the time of an ordinary run.
func buildBinary(test *testing.T) string {
	test.Helper()
	goTool, err := exec.LookPath("go")
	if err != nil {
		test.Fatalf("the go tool, which builds the binary under test, is not on the PATH: %v", err)
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	binary := filepath.Join(test.TempDir(), "doubleagent"+suffix)
	command := exec.Command(goTool, "build", "-o", binary, "./cmd/doubleagent")
	command.Dir = repositoryRoot(test)
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := command.CombinedOutput(); err != nil {
		test.Fatalf("building the binary with %s: %v\n%s", goTool, err, output)
	}
	return binary
}

// execute runs the binary's collect with the invocation and reads what it
// wrote.
func execute(test testing.TB, binary string, run invocation) collectorRun {
	test.Helper()
	command := exec.Command(binary, append([]string{"collect"}, run.args...)...)
	command.Dir = run.cwd
	command.Env = append(os.Environ(), run.env...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	result := collectorRun{files: map[string]string{}, outDir: run.outDir}
	var exitErr *exec.ExitError
	if err := command.Run(); errors.As(err, &exitErr) {
		result.code = exitErr.ExitCode()
	} else if err != nil {
		test.Fatal(err)
	}
	result.stdout, result.stderr = stdout.String(), stderr.String()
	result.name = runName(run.outDir)
	if result.name != "" {
		readOutputs(test, &result)
	}
	return result
}

// runName returns the run name of the output in dir: the log's base name
// without its suffix, or "" when the directory holds none. It lists the
// directory rather than matching a pattern, since the directory's own name
// may hold pattern characters.
func runName(dir string) string {
	names := runFileNames(dir, ".log")
	if len(names) != 1 {
		return ""
	}
	return strings.TrimSuffix(names[0], ".log")
}

// runFileNames returns the names in dir that end in suffix and hold the
// "_agent-artifacts." marker of a run's output files.
func runFileNames(dir, suffix string) []string {
	if dir == "" {
		return nil
	}
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.Contains(name, "_agent-artifacts.") && strings.HasSuffix(name, suffix) {
			names = append(names, name)
		}
	}
	return names
}

// readOutputs fills the run with the text of its output files and its
// archive's members.
func readOutputs(test testing.TB, run *collectorRun) {
	test.Helper()
	for _, suffix := range []string{".manifest.jsonl", ".collection.json", ".log"} {
		data, err := os.ReadFile(filepath.Join(run.outDir, run.name+suffix))
		if err != nil {
			test.Errorf("the run wrote no %s: %v", suffix, err)
		}
		run.files[suffix] = string(data)
	}
	members, err := readArchive(filepath.Join(run.outDir, run.name+".tar.gz"))
	if err != nil {
		test.Errorf("archive: %v", err)
	}
	run.members = members
}

// readArchive reads the gzip-compressed tar at path and returns its members
// in archive order.
func readArchive(path string) ([]archiveMember, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	unzipped, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	reader := tar.NewReader(unzipped)
	var members []archiveMember
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return members, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		member, err := describeMember(header, reader)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		members = append(members, member)
	}
}

// describeMember returns the member the reader is at.
func describeMember(header *tar.Header, reader io.Reader) (archiveMember, error) {
	member := archiveMember{name: header.Name}
	switch header.Typeflag {
	case tar.TypeReg:
		content, err := io.ReadAll(reader)
		if err != nil {
			return member, err
		}
		member.description = hashText(string(content))
		if _, holdsCopy := archivedCopies[header.Name]; holdsCopy {
			member.text = string(content)
		}
	case tar.TypeSymlink, tar.TypeLink:
		member.description = "link " + header.Linkname
	default:
		member.description = fmt.Sprintf("type %c", header.Typeflag)
	}
	return member, nil
}

// hashText returns the hex SHA-256 of text.
func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// newGoldenRun builds the binary and the image the cases share, with the
// umask pinned so the image's modes do not depend on the tests run before.
func newGoldenRun(test *testing.T) goldenRun {
	test.Helper()
	pinUmask(test)
	return goldenRun{binary: buildBinary(test), image: imagetest.Build(test, test.TempDir())}
}

// placeholders returns the replacements of a case's placeholders: {ROOT},
// {WORK}, {ALICE}, {BOB} and {OUT}, the image root, the image's work
// directory, the two homes the cases name and the run's output directory.
func placeholders(img imagetest.Image, outDir string) *strings.Replacer {
	return strings.NewReplacer("{ROOT}", img.Root, "{WORK}", img.Work,
		"{ALICE}", img.Alice, "{BOB}", img.Bob, "{OUT}", outDir)
}
