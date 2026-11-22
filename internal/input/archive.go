package input

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// openArchive extracts the archive at source into a new directory and
// returns its collection: kind archive when it unpacked to manifest.jsonl
// and fs/ at the top, loose otherwise. A `.sha256` file beside the archive
// is checked, its result a note. Messages name the archive, its sha256
// file and the new directory as names gives them, the new directory's
// name added to it.
func openArchive(ctx context.Context, source, workDir string, names *inputNames) (*Collection, error) {
	// Opened once first, so an unreadable archive fails with its reason
	// before a directory is made.
	file, err := osfile.Open(source)
	if err != nil {
		return nil, err
	}
	_ = file.Close()
	destination, err := makeExtractionDir(workDir)
	if err != nil {
		return nil, err
	}
	if workDir == "" {
		names.add(destination, destination)
	} else {
		names.add(destination, filepath.Join(workDir, filepath.Base(destination)))
	}
	collection := &Collection{Source: source, Kind: KindArchive, Root: destination, extracted: destination}
	shown := names.path(source)
	sidecar := source + ".sha256"
	if info, err := osfile.Stat(sidecar); err == nil && info.Mode().IsRegular() {
		note, err := verifySidecar(source, sidecar, shown+".sha256")
		if err != nil {
			_ = collection.Cleanup()
			return nil, err
		}
		collection.Notes = append(collection.Notes, note)
	}
	out, err := extractArchive(ctx, source, shown, destination)
	if err != nil {
		_ = collection.Cleanup()
		return nil, err
	}
	for _, problem := range out.problems {
		collection.addProblem(problem)
	}
	collection.extractedFiles = map[string]bool{}
	for key, directory := range out.made {
		if !directory {
			collection.extractedFiles[key] = true
		}
	}
	if !isCollectionLayout(destination) {
		collection.Kind = KindLoose
	}
	return collection, nil
}

// makeExtractionDir creates a new directory with a random name, prefixed
// doubleagent-, under workDir (created when missing) or the system's
// temporary directory, and returns its absolute path.
func makeExtractionDir(workDir string) (string, error) {
	base, pattern := workDir, "doubleagent-*"
	if workDir == "" {
		base = os.TempDir()
	} else if err := osfile.MkdirAll(workDir, 0o777); err != nil {
		return "", err
	}
	base, err := absolute(base)
	if err != nil {
		return "", err
	}
	return osfile.MkdirTemp(base, pattern)
}

// verifySidecar returns the note for the archive's sha256 file, which a
// note names as shownSidecar: verified, a warning that the digests differ,
// or a warning that the file could not be read. The expected digest is the
// file's first run of characters up to ASCII whitespace. An archive that
// cannot be read is an error.
func verifySidecar(archive, sidecar, shownSidecar string) (string, error) {
	data, err := osfile.ReadFile(sidecar)
	if err != nil {
		return "WARNING archive sha256 not checked: " + describeError(shownSidecar, err), nil
	}
	expected := ""
	if fields := strings.FieldsFunc(textlines.ValidUTF8(data), isASCIIWhitespace); len(fields) > 0 {
		expected = strings.ToLower(fields[0])
	}
	file, err := osfile.Open(archive)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", &fs.PathError{Op: "read", Path: archive, Err: err}
	}
	actual := hex.EncodeToString(digest.Sum(nil))
	name := filepath.Base(sidecar)
	if actual == expected {
		return "archive sha256 verified against " + name, nil
	}
	return "WARNING archive sha256 " + actual + " does not match " + name + " (" + expected + ")", nil
}

// isASCIIWhitespace reports whether character is ASCII whitespace: space,
// tab, CR, LF, form feed or vertical tab.
func isASCIIWhitespace(character rune) bool {
	return strings.ContainsRune(" \t\r\n\f\v", character)
}
