//go:build parity

package input

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// parityPython returns the interpreter PARITY_PYTHON names and the
// analyzer directory, skipping the test when the variable is unset.
func parityPython(t *testing.T) (python, analyzer string) {
	t.Helper()
	python = os.Getenv("PARITY_PYTHON")
	if python == "" {
		t.Skip("PARITY_PYTHON is not set; it names the Python 3 interpreter the oracle runs on")
	}
	analyzer, err := filepath.Abs(filepath.Join("..", "..", "analyzer"))
	if err != nil {
		t.Fatal(err)
	}
	return python, analyzer
}

// runPython runs a script of testdata with arguments and returns its
// standard output.
func runPython(t *testing.T, script string, arguments ...string) string {
	t.Helper()
	python, analyzer := parityPython(t)
	scriptPath, err := filepath.Abs(filepath.Join("testdata", script))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(python, append([]string{"-I", scriptPath, analyzer}, arguments...)...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("%s: %v\n%s", script, err, stderr.String())
	}
	return string(output)
}

// normalised returns a dump with the reason of an unreadable
// collection.json taken out: its wording is the binary's own (documented
// difference 52).
func normalised(dump string) string {
	lines := strings.Split(dump, "\n")
	for i, line := range lines {
		if prefix := "note\tcollection.json unreadable: "; strings.HasPrefix(line, prefix) {
			lines[i] = prefix + "<reason>"
		}
	}
	return strings.Join(lines, "\n")
}

// compareWithPython opens input with both analyzers and fails the test
// when the dumps differ.
func compareWithPython(t *testing.T, input, catalogText, host string) {
	t.Helper()
	arguments := []string{input}
	if catalogText != "" {
		catalogFile := filepath.Join(t.TempDir(), "catalog.txt")
		writeFile(t, catalogFile, catalogText)
		arguments = append(arguments, "--catalog", catalogFile)
	}
	if host != "" {
		arguments = append(arguments, "--host", host)
	}
	want := runPython(t, "dump_input.py", arguments...)
	got := "error\n"
	collection, err := Open(input, Options{Matcher: matcherOf(t, catalogText), Host: host})
	if err == nil {
		got = dumpCollection(collection)
		if err := collection.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}
	if strings.HasPrefix(want, "kind\tloose\n") {
		// The binary reads a loose tree's directories in name order and
		// the analyzer in the file system's, so a loose input is compared
		// as a sorted sequence of lines; a collected input keeps the
		// manifest's order and is compared in order.
		got, want = listingOrder(got), listingOrder(want)
	}
	if normalised(got) != normalised(want) {
		t.Fatalf("Go and Python differ for %s\n--- Go\n%s--- Python\n%s", input, got, want)
	}
}

// listingOrder returns a dump with each home's index in the home, staged
// and artifact lines replaced by the home's directory, so that two
// listings of one tree name the same home alike, and its lines sorted.
func listingOrder(dump string) string {
	lines := strings.Split(strings.TrimSuffix(dump, "\n"), "\n")
	homes := map[string]string{}
	for _, line := range lines {
		if fields := strings.Split(line, "\t"); fields[0] == "home" && len(fields) > 2 {
			homes[fields[1]] = fields[2]
		}
	}
	for i, line := range lines {
		fields := strings.Split(line, "\t")
		switch fields[0] {
		case "home", "staged", "artifact":
			if len(fields) > 1 {
				fields[1] = homes[fields[1]]
			}
		}
		lines[i] = strings.Join(fields, "\t")
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n") + "\n"
}

func TestOpenMatchesPythonOnEachCase(t *testing.T) {
	parityPython(t)
	for _, test := range allInputCases() {
		t.Run(test.name, func(t *testing.T) {
			if test.pythonDiffers {
				t.Skip("Python crashes on this input or reads it differently by a documented choice")
			}
			compareWithPython(t, test.build(t, t.TempDir()), test.catalog, test.host)
		})
	}
}

func TestOpenMatchesPythonOnTheFixtureInputs(t *testing.T) {
	parityPython(t)
	listing := runPython(t, "make_inputs.py", t.TempDir())
	for _, line := range strings.Split(strings.TrimSpace(listing), "\n") {
		name, input, found := strings.Cut(line, "\t")
		if !found {
			t.Fatal(errors.New("bad listing line " + line))
		}
		t.Run(name, func(t *testing.T) {
			compareWithPython(t, input, "", "")
		})
	}
}

// dumpField returns text's bytes with every byte outside printable ASCII,
// and the backslash, as \xHH, as testdata/dump_input.py prints a field.
func dumpField(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		if character := text[i]; character >= 0x20 && character <= 0x7e && character != '\\' {
			out.WriteByte(character)
		} else {
			fmt.Fprintf(&out, `\x%02x`, character)
		}
	}
	return out.String()
}

// dumpCollection returns the collection in the form dump_input.py prints
// Python's: kind, host, notes, problems, homes with their staged paths, and
// artifacts, a paths under the extraction directory starting <extract>.
func dumpCollection(collection *Collection) string {
	extracted := collection.ExtractedDir()
	path := func(text string) string {
		if extracted != "" && (text == extracted || strings.HasPrefix(text, extracted+string(filepath.Separator))) {
			text = "<extract>" + text[len(extracted):]
		}
		return dumpField(text)
	}
	lines := []string{"kind\t" + collection.Kind, "host\t" + dumpField(collection.Host)}
	for _, note := range collection.Notes {
		lines = append(lines, "note\t"+dumpField(note))
	}
	for _, problem := range collection.Problems {
		lines = append(lines, "problem\t"+dumpField(problem))
	}
	index := map[*Home]int{}
	for i, home := range collection.Homes {
		index[home] = i
		lines = append(lines, strings.Join([]string{"home", strconv.Itoa(i), path(home.DiskPath),
			dumpField(home.Original), dumpField(home.User), dumpField(home.Host),
			strconv.FormatBool(home.Inferred), strconv.FormatBool(home.Project)}, "\t"))
		rels := make([]string, 0, len(home.StagedPaths))
		for rel := range home.StagedPaths {
			rels = append(rels, rel)
		}
		slices.Sort(rels)
		for _, rel := range rels {
			lines = append(lines, strings.Join([]string{"staged", strconv.Itoa(i), dumpField(rel),
				path(home.StagedPaths[rel])}, "\t"))
		}
	}
	for _, artifact := range collection.Artifacts {
		manifest := "-"
		if artifact.Manifest.IsObject() {
			manifest = dumpField(artifact.Manifest.JSON())
		}
		lines = append(lines, strings.Join([]string{"artifact", strconv.Itoa(index[artifact.Home]),
			dumpField(artifact.Agent), dumpField(artifact.Rel), dumpField(artifact.Original),
			path(artifact.DiskPath), manifest}, "\t"))
	}
	return strings.Join(lines, "\n") + "\n"
}
