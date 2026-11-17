package input

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// tempNameCharacters are the characters of a new extraction directory's
// random name, the set Python's tempfile draws from.
const tempNameCharacters = "abcdefghijklmnopqrstuvwxyz0123456789_"

// openArchive extracts the archive at source into a new directory and
// returns its collection: kind archive when it unpacked to manifest.jsonl
// and fs/ at the top, loose otherwise. A `.sha256` file beside the archive
// is checked, its result a note.
func openArchive(source, workDir string) (*Collection, error) {
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
	collection := &Collection{Source: source, Kind: KindArchive, Root: destination, extracted: destination}
	sidecar := source + ".sha256"
	if info, err := osfile.Stat(sidecar); err == nil && info.Mode().IsRegular() {
		note, err := verifySidecar(source, sidecar)
		if err != nil {
			_ = collection.Cleanup()
			return nil, err
		}
		collection.Notes = append(collection.Notes, note)
	}
	if err := extractArchive(source, destination); err != nil {
		_ = collection.Cleanup()
		return nil, err
	}
	if !isCollectionLayout(destination) {
		collection.Kind = KindLoose
	}
	return collection, nil
}

// makeExtractionDir creates a new directory with a random name, prefixed
// cac- under workDir (created when missing) or cac-analyzer- under the
// system's temporary directory, and returns its absolute path. A name is
// taken only when nothing exists there; the name's randomness keeps another
// process from making it first.
func makeExtractionDir(workDir string) (string, error) {
	base, prefix := workDir, "cac-"
	if workDir == "" {
		base, prefix = os.TempDir(), "cac-analyzer-"
	} else if err := osfile.MkdirAll(workDir, 0o777); err != nil {
		return "", err
	}
	if !filepath.IsAbs(base) {
		working, err := os.Getwd()
		if err != nil {
			return "", err
		}
		base = filepath.Join(working, base)
	}
	for range 100 {
		path := cleanPath(filepath.Join(base, prefix+randomName()))
		if _, err := osfile.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err := osfile.MkdirAll(path, 0o700); err != nil {
			return "", err
		}
		if info, err := osfile.Lstat(path); err != nil || !info.IsDir() {
			return "", &fs.PathError{Op: "mkdir", Path: path, Err: fs.ErrExist}
		}
		return path, nil
	}
	return "", &fs.PathError{Op: "mkdir", Path: filepath.Join(base, prefix), Err: fs.ErrExist}
}

// randomName returns sixteen characters drawn at random from
// tempNameCharacters.
func randomName() string {
	name := make([]byte, 16)
	_, _ = rand.Read(name)
	for i, value := range name {
		name[i] = tempNameCharacters[int(value)%len(tempNameCharacters)]
	}
	return string(name)
}

// verifySidecar returns the note for the archive's sha256 file: verified,
// a warning that the digests differ, or a warning that the file could not
// be read. The expected digest is the file's first run of characters up to
// ASCII whitespace. An archive that cannot be read is an error.
func verifySidecar(archive, sidecar string) (string, error) {
	data, err := osfile.ReadFile(sidecar)
	if err != nil {
		return "WARNING archive sha256 not checked: " + describeError(sidecar, err), nil
	}
	expected := ""
	if fields := strings.FieldsFunc(pytext.DecodeUTF8(data), isASCIIWhitespace); len(fields) > 0 {
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
	name := parsePath(sidecar).name()
	if actual == expected {
		return "archive sha256 verified against " + name, nil
	}
	return "WARNING archive sha256 " + actual + " does not match " + name + " (" + expected + ")", nil
}

// isASCIIWhitespace reports whether character is one of Python's ASCII
// whitespace characters.
func isASCIIWhitespace(character rune) bool {
	return strings.ContainsRune(" \t\r\n\f\v", character)
}
