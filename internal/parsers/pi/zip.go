package pi

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"errors"
	"io"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// maxInflated is how many bytes the members of one bug report zip may
// decompress to, together. A member's declared size is the archive's own
// claim, so the reader stops at this bound whatever the archive says; a
// real report (two small JSON files and one session transcript or summary)
// is far below it.
const maxInflated = 256 << 20

// errTooLarge is the reason a member past maxInflated is not read.
var errTooLarge = errors.New("it decompresses past the " + strconv.Itoa(maxInflated>>20) +
	" MiB read from one bug report")

// errBeforeStart is the reason a member is not read when the archive's
// directory places its header before the archive's first byte.
var errBeforeStart = errors.New("header placed before the start of the archive")

// archiveBytes is an archive held in memory, read at an offset with the
// reason errBeforeStart for an offset before its start.
type archiveBytes struct {
	data *bytes.Reader
}

// ReadAt reads len(into) bytes at offset, as io.ReaderAt says.
func (a archiveBytes) ReadAt(into []byte, offset int64) (int, error) {
	if offset < 0 {
		return 0, errBeforeStart
	}
	return a.data.ReadAt(into, offset)
}

// bugMembers returns the members of a bug report zip that are read, the
// two always written first.
func bugMembers() []string {
	return []string{"report.json", "diagnostics.json", "session.jsonl", "summary.md"}
}

// readMember returns a member's decompressed content, an error when it is
// damaged or would take more than budget bytes.
func readMember(file *zip.File, budget int) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	content, err := io.ReadAll(io.LimitReader(reader, int64(budget)+1))
	if closeErr := reader.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	if len(content) > budget {
		return nil, errTooLarge
	}
	return content, nil
}

// zipReason returns the reason an archive or member could not be read, in
// the binary's own words.
func zipReason(err error) string {
	var corrupt flate.CorruptInputError
	switch {
	case errors.Is(err, errTooLarge), errors.Is(err, errBeforeStart):
		return err.Error()
	case errors.Is(err, zip.ErrFormat):
		return "not a valid zip archive"
	case errors.Is(err, zip.ErrAlgorithm):
		return "unsupported compression method"
	case errors.Is(err, zip.ErrChecksum):
		return "checksum mismatch"
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "data cut off"
	case errors.As(err, &corrupt):
		return "damaged compressed data at byte " + strconv.FormatInt(int64(corrupt), 10)
	}
	return err.Error()
}

// zipMembers returns the known members of the bug report zip rel held in
// data, decompressing at most budget bytes in all, as bugZipMembers says.
func zipMembers(rel string, data []byte, budget int, problems *parserkit.Problems) map[string][]byte {
	archive, err := zip.NewReader(archiveBytes{data: bytes.NewReader(data)}, int64(len(data)))
	if err != nil && !(errors.Is(err, zip.ErrInsecurePath) && archive != nil) {
		problems.Add(0, rel+" is not a zip archive ("+zipReason(err)+")")
		return nil
	}
	files := map[string]*zip.File{}
	for _, file := range archive.File {
		files[file.Name] = file
	}
	members := map[string][]byte{}
	for i, name := range bugMembers() {
		file, ok := files[name]
		if !ok {
			if i < 2 {
				problems.Add(0, rel+" has no member "+name)
			}
			continue
		}
		content, err := readMember(file, budget)
		if err != nil {
			problems.Add(0, rel+" member "+name+" could not be read ("+zipReason(err)+")")
			continue
		}
		budget -= len(content)
		members[name] = content
	}
	return members
}

// bugZipMembers returns the known members of a bug report zip by name,
// read in memory and never written to disk. An archive that cannot be
// opened, a missing report.json or diagnostics.json, and a member that
// cannot be read each add a problem. Of two members with one name, the
// later is read.
func bugZipMembers(artifact *input.Artifact, problems *parserkit.Problems) map[string][]byte {
	data, ok := parserkit.FileBytes(artifact.DiskPath, problems)
	if !ok {
		return nil
	}
	return zipMembers(artifact.Rel, data, maxInflated, problems)
}
