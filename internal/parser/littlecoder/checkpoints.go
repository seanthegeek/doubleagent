package littlecoder

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/pi"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// absentSuffix ends the name of the empty marker of a file that did not
// exist before the session first edited it.
const absentSuffix = ".absent"

// regularFiles returns the regular files of a directory sorted by name,
// none when it cannot be listed; a link is not a regular file.
func regularFiles(directory string) []os.DirEntry {
	entries, err := parser.ReadDir(directory)
	if err != nil {
		return nil
	}
	files := slices.DeleteFunc(entries, func(entry os.DirEntry) bool { return !entry.Type().IsRegular() })
	slices.SortFunc(files, func(first, second os.DirEntry) int {
		return strings.Compare(first.Name(), second.Name())
	})
	return files
}

// parseCheckpoints emits the system row of a checkpoint directory, on the
// directory's first file by name that it is given (rowFile), so each
// directory gives one row. Once a directory is listed, its other files are
// passed over by name, and every file of one that could not be listed or
// holds no such file ("" remembered) gives nothing.
func (p *Parser) parseCheckpoints(artifact *input.Artifact, emit parser.Emit) {
	setName := p.checkpoint.FindStringSubmatch(artifact.Rel)[1]
	key := checkpointKey{set: setName}
	if artifact.Home != nil {
		key.home = artifact.Home.DiskPath
	}
	if first, held := p.firstCheckpoint[key]; held && first != filepath.Base(artifact.DiskPath) {
		return
	}
	directory, found := parser.Beside(artifact)
	if !found {
		return
	}
	p.Work.Add("checkpoint listings", 1)
	files := regularFiles(directory)
	first, named := p.rowFile(files, setName)
	p.firstCheckpoint[key] = first
	if !named || first != filepath.Base(artifact.DiskPath) {
		return
	}
	mtimes, listed := describeFiles(files)
	row := p.BaseRow(artifact)
	row.TurnType = timeline.TurnSystem
	header, failure := jsonvalue.NewNull(), parser.Problems(nil)
	what := "no session file (" + setName + ")"
	if match := p.sessionFile.FindStringSubmatch(setName); match != nil {
		row.SessionID, what = match[1], "pi session "+setName
		header, failure = p.sessionHeader(artifact, setName)
	}
	if header.IsObject() {
		row.SessionID = timeutil.FirstNonEmpty(parser.Text(header.Get("id")), row.SessionID)
		row.ProjectPath = parser.Text(header.Get("cwd"))
	}
	if len(mtimes) > 0 {
		row.TimestampUTC = timeutil.FromEpoch(slices.Min(mtimes))
	}
	collected := ""
	if !header.IsObject() && len(failure) == 0 {
		collected = ", session file not collected"
	}
	row.Text = timeline.Compact("little-coder checkpoints for " + what + ": " +
		strconv.Itoa(len(listed)) + " pre-edit file(s)" + collected + ": " + strings.Join(listed, "; "))
	emit(row)
	parser.EmitAll(emit, p.ErrorRows(artifact, failure, row.SessionID, row.ProjectPath))
}

// rowFile returns the name of the file of a checkpoint directory, among
// its regular files sorted by name, that gives the directory's row: the
// first, passing over a snapshots.json that Parse reads as v0.0.x
// snapshots (in a directory not named after a pi session file), which
// never reaches parseCheckpoints. It is false when there is none.
func (p *Parser) rowFile(files []os.DirEntry, setName string) (string, bool) {
	snapshots := !p.sessionFile.MatchString(setName)
	for _, file := range files {
		if !snapshots || file.Name() != "snapshots.json" {
			return file.Name(), true
		}
	}
	return "", false
}

// describeFiles returns the modification times of the files that could be
// looked at, and each file's name as the row lists it: an absence marker
// as the name it stands for, "(did not exist)" after it.
func describeFiles(files []os.DirEntry) ([]float64, []string) {
	var mtimes []float64
	var listed []string
	for _, entry := range files {
		if info, err := entry.Info(); err == nil {
			mtimes = append(mtimes, parser.MtimeSeconds(info))
		}
		if name, absent := strings.CutSuffix(entry.Name(), absentSuffix); absent {
			listed = append(listed, name+" (did not exist)")
		} else {
			listed = append(listed, entry.Name())
		}
	}
	return mtimes, listed
}

// lstatPresent returns the entry at path, nil and no error when nothing is
// there, or the error that stopped the look.
func lstatPresent(path string) (fs.FileInfo, error) {
	info, err := osfile.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return nil, nil
	}
	return info, err
}

// isDirectory reports whether an entry is a directory, a link never
// counting.
func isDirectory(info fs.FileInfo) bool {
	return info != nil && info.IsDir()
}

// projectSession looks for the session file name in one project directory
// of sessions. It returns the file's header and whole-file problems and
// true when the directory holds it, or the problem of a look that failed
// and false.
func projectSession(directory, sessions, name string) (jsonvalue.Value, parser.Problems, bool) {
	info, err := lstatPresent(directory)
	if err == nil && !isDirectory(info) {
		return jsonvalue.NewNull(), nil, false
	}
	failed := sessions
	file, named := "", false
	if err == nil {
		failed = directory
		if file, named = parser.Under(directory, name); named {
			info, err = lstatPresent(file)
		}
	}
	if err != nil {
		return jsonvalue.NewNull(), parser.Problems{{Message: reason.Unreadable(failed, err)}}, false
	}
	if !named || info == nil || !info.Mode().IsRegular() {
		return jsonvalue.NewNull(), nil, false
	}
	session := pi.ReadSession(file)
	// A problem without raw text concerns the whole file (it could not be
	// read); a bad line has its raw text and the pi parser reports it.
	whole := slices.DeleteFunc(slices.Clone(session.Problems), func(problem parser.Problem) bool {
		return problem.Raw != ""
	})
	return session.Header, whole, true
}
