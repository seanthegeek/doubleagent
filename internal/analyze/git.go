package analyze

import (
	"slices"

	"github.com/seanthegeek/doubleagent/internal/gitmeta"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// projectGit is the remotes and the checked-out branch or commit of one
// project directory's repository.
type projectGit struct {
	remotes []gitmeta.Remote
	head    string
}

// gitFile is one of the two repository files chosen for a project: which
// field it fills and the artifact holding it.
type gitFile struct {
	head     bool
	artifact *input.Artifact
}

// projectRepositories returns the repository of each project directory
// among the artifacts that holds .git/config or .git/HEAD, keyed by the
// project's path as projectGroups keys it. git's own spelling of a file wins,
// else the first spelled otherwise in ASCII case, since a collection may keep
// the on-disk spelling. A file that cannot be read, and a value git would
// refuse, adds "<original path>: <reason>" to problems once.
func projectRepositories(artifacts []*input.Artifact, problems *[]string) map[string]projectGit {
	var order []string
	chosen := map[string][]gitFile{}
	for _, artifact := range artifacts {
		if !artifact.Home.Project || artifact.Agent != input.ProjectAgent {
			continue
		}
		lower := textlines.LowerASCII(artifact.Rel)
		var head bool
		switch lower {
		case gitmeta.ConfigRel:
		case textlines.LowerASCII(gitmeta.HeadRel):
			head = true
		default:
			continue
		}
		exact := artifact.Rel == gitmeta.ConfigRel || artifact.Rel == gitmeta.HeadRel
		path := artifact.Home.ListedPath()
		if _, ok := chosen[path]; !ok {
			order = append(order, path)
			chosen[path] = nil
		}
		chosen[path] = chooseGitFile(chosen[path], gitFile{head: head, artifact: artifact}, exact)
	}
	known := map[string]bool{}
	for _, line := range *problems {
		known[line] = true
	}
	repositories := map[string]projectGit{}
	for _, path := range order {
		repositories[path] = readRepository(chosen[path], problems, known)
	}
	return repositories
}

// chooseGitFile returns files with file added, or put in place of the file
// chosen for its field when file has git's own spelling and that one has
// not; a field keeps its first place among the files.
func chooseGitFile(files []gitFile, file gitFile, exact bool) []gitFile {
	index := slices.IndexFunc(files, func(each gitFile) bool { return each.head == file.head })
	switch {
	case index < 0:
		return append(files, file)
	case exact && files[index].artifact.Rel != file.artifact.Rel:
		files[index] = file
	}
	return files
}

// readRepository reads the chosen files of one project into its remotes and
// head, adding each problem line not already in known, the set of the
// lines of problems.
func readRepository(files []gitFile, problems *[]string, known map[string]bool) projectGit {
	var repository projectGit
	for _, file := range files {
		text, failure := readGitText(file.artifact.DiskPath)
		var messages []string
		switch {
		case failure != "":
			messages = []string{failure}
		case file.head:
			repository.head = gitmeta.ParseHead(text)
		default:
			repository.remotes, messages = gitmeta.ParseConfig(text)
		}
		for _, message := range messages {
			line := file.artifact.Original + ": " + message
			if !known[line] {
				known[line] = true
				*problems = append(*problems, line)
			}
		}
	}
	if repository.remotes == nil {
		repository.remotes = []gitmeta.Remote{}
	}
	return repository
}

// readGitText returns a whole repository file as text, invalid UTF-8
// replaced, or why it cannot be read. parserkit.ReadFile reads only a
// regular file, so a link is never followed.
func readGitText(path string) (string, string) {
	data, err := parserkit.ReadFile(path)
	if err != nil {
		return "", reason.For(err)
	}
	return textlines.ValidUTF8(data), ""
}
