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
// directory's first file by name, so each directory gives one row.
func (p *Parser) parseCheckpoints(artifact *input.Artifact, emit parser.Emit) {
	setName := p.checkpoint.FindStringSubmatch(artifact.Rel)[1]
	files := regularFiles(parser.Beside(artifact))
	if len(files) == 0 || files[0].Name() != filepath.Base(artifact.DiskPath) {
		return
	}
	mtimes, listed := describeFiles(files)
	row := p.BaseRow(artifact)
	row.TurnType = timeline.TurnSystem
	header, failure := jsonvalue.NewNull(), parser.Problems(nil)
	what := "no session file (" + setName + ")"
	if match := p.sessionFile.FindStringSubmatch(setName); match != nil {
		row.SessionID, what = match[1], "pi session "+setName
		header, failure = sessionHeader(artifact, setName)
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

// sessionHeader returns the header of the pi session file name in a project
// directory of .pi/agent/sessions, null when none is found, and the
// whole-file problems of the search: the sessions directory that could not
// be listed, or the first project directory or session file that could not
// be looked at when no directory held the file (a session file name
// belongs to one project), or the session file that could not be read.
func sessionHeader(artifact *input.Artifact, name string) (jsonvalue.Value, parser.Problems) {
	// The sessions directory is a fixed path of the home, not a neighbour
	// of the checkpoint: Home.PathOf finds it where the collection staged
	// it, at a symbolic link's physical path, which climbing from the
	// checkpoint's own directory would miss.
	if artifact.Home == nil {
		return jsonvalue.NewNull(), nil
	}
	sessions := artifact.Home.PathOf(".pi/agent/sessions")
	var directories []string
	info, err := lstatPresent(sessions)
	if err == nil && isDirectory(info) {
		var entries []os.DirEntry
		if entries, err = parser.ReadDir(sessions); err == nil {
			for _, entry := range entries {
				directories = append(directories, entry.Name())
			}
			slices.Sort(directories)
		}
	}
	if err != nil {
		return jsonvalue.NewNull(), parser.Problems{{Message: reason.Unreadable(sessions, err)}}
	}
	var firstFailure parser.Problems
	for _, directory := range directories {
		header, failure, found := projectSession(filepath.Join(sessions, directory), sessions, name)
		if found {
			return header, failure
		}
		if firstFailure == nil {
			firstFailure = failure
		}
	}
	return jsonvalue.NewNull(), firstFailure
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
	if err == nil {
		failed = directory
		info, err = lstatPresent(filepath.Join(directory, name))
	}
	if err != nil {
		return jsonvalue.NewNull(), parser.Problems{{Message: reason.Unreadable(failed, err)}}, false
	}
	if info == nil || !info.Mode().IsRegular() {
		return jsonvalue.NewNull(), nil, false
	}
	session := pi.ReadSession(filepath.Join(directory, name))
	// A problem without raw text concerns the whole file (it could not be
	// read); a bad line has its raw text and the pi parser reports it.
	whole := slices.DeleteFunc(slices.Clone(session.Problems), func(problem parser.Problem) bool {
		return problem.Raw != ""
	})
	return session.Header, whole, true
}
