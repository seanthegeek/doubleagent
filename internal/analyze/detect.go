package analyze

import (
	"context"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/gitmeta"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// fileCounts is the number and total size of one agent's files. A total
// size beyond the largest int64 is held at that largest value (addBytes).
type fileCounts struct {
	agent        string
	files, bytes int64
}

// agentFiles is one agent of a home or project in detect --json and
// inventory.json: its file counts and whether a parser reads it.
type agentFiles struct {
	fileCounts
	parser bool
}

// homeEntry is one home of the homes array: a home that is not a project
// directory, with its agents sorted by name.
type homeEntry struct {
	user, home string
	inferred   bool
	agents     []agentFiles
}

// projectEntry is one project directory of the input: its path on the
// source host, the users whose files it holds (sorted), its agents sorted
// by name, the files of agent project that no single agent owns, and its
// repository.
type projectEntry struct {
	path                     string
	users                    []string
	agents                   []agentFiles
	sharedFiles, sharedBytes int64
	git                      projectGit
}

// detection is what detect --json and inventory.json share.
type detection struct {
	input, kind, host string
	notes             []string
	homes             []homeEntry
	projects          []projectEntry
}

// parserAgents returns the agents a parser of the registry reads.
func parserAgents(registry *parsers.Registry) map[string]bool {
	agents := map[string]bool{}
	for agent := range parsers.ByAgent(registry.All()) {
		agents[agent] = true
	}
	return agents
}

// artifactBytes returns an artifact's manifest size, or for loose input the
// size of its own entry, 0 when it cannot be read.
func artifactBytes(artifact *input.Artifact) int64 {
	if !artifact.Manifest.IsNull() {
		return artifact.Size()
	}
	info, err := osfile.Lstat(artifact.DiskPath)
	if err != nil {
		return 0
	}
	return info.Size()
}

// agentCounts are the file counts of the agents of one home or project, in
// the order the agents were first met.
type agentCounts = firstMet[string, fileCounts]

// countAgent adds one artifact of size bytes to the counts of its agent.
func countAgent(counts *agentCounts, agent string, size int64) {
	entry := counts.entry(agent, fileCounts{agent: agent})
	entry.files++
	entry.bytes = addBytes(entry.bytes, size)
}

// addBytes returns total plus size, two sizes that are never negative, held
// at the largest int64 rather than wrapping to a negative number: a
// manifest may state any size up to that value for each file.
func addBytes(total, size int64) int64 {
	if size > math.MaxInt64-total {
		return math.MaxInt64
	}
	return total + size
}

// withParsers returns counts sorted by agent, each marked with whether a
// parser reads it.
func withParsers(counts []fileCounts, parsed map[string]bool) []agentFiles {
	sorted := slices.Clone(counts)
	slices.SortFunc(sorted, func(left, right fileCounts) int { return strings.Compare(left.agent, right.agent) })
	agents := make([]agentFiles, len(sorted))
	for i, counts := range sorted {
		agents[i] = agentFiles{fileCounts: counts, parser: parsed[counts.agent]}
	}
	return agents
}

// homeAgents returns one entry per home that is not a project directory, in
// the collection's order, each with the agents it holds files of.
func homeAgents(collection *input.Collection, parsed map[string]bool) []homeEntry {
	counts := map[*input.Home]*agentCounts{}
	for _, artifact := range collection.Artifacts {
		if !artifact.Home.Project {
			if counts[artifact.Home] == nil {
				counts[artifact.Home] = &agentCounts{}
			}
			countAgent(counts[artifact.Home], artifact.Agent, artifactBytes(artifact))
		}
	}
	var homes []homeEntry
	for _, home := range collection.Homes {
		if !home.Project {
			var agents []fileCounts
			if counts[home] != nil {
				agents = counts[home].values
			}
			homes = append(homes, homeEntry{user: home.User, home: home.Original,
				inferred: home.Inferred, agents: withParsers(agents, parsed)})
		}
	}
	return homes
}

// projectGroups returns the project directories of the input sorted by
// path: their users, their agents' files and their shared files.
func projectGroups(collection *input.Collection, parsed map[string]bool) []projectEntry {
	groups := map[string]*projectEntry{}
	counts := map[string]*agentCounts{}
	users := map[string]map[string]bool{}
	for _, artifact := range collection.Artifacts {
		if !artifact.Home.Project {
			continue
		}
		path := artifact.Home.ListedPath()
		group, ok := groups[path]
		if !ok {
			group = &projectEntry{path: path, users: []string{}}
			groups[path] = group
			counts[path], users[path] = &agentCounts{}, map[string]bool{}
		}
		if user := artifact.Home.User; user != "" && !users[path][user] {
			users[path][user] = true
			group.users = append(group.users, user)
		}
		size := artifactBytes(artifact)
		if artifact.Agent == input.ProjectAgent {
			group.sharedFiles++
			group.sharedBytes = addBytes(group.sharedBytes, size)
		} else {
			countAgent(counts[path], artifact.Agent, size)
		}
	}
	projects := make([]projectEntry, 0, len(groups))
	for _, path := range sortedKeys(groups) {
		group := groups[path]
		slices.Sort(group.users)
		group.agents = withParsers(counts[path].values, parsed)
		projects = append(projects, *group)
	}
	return projects
}

// sortedKeys returns the keys of a map in byte order, which for UTF-8 text
// is code point order.
func sortedKeys[V any](entries map[string]V) []string {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// detect returns what detect --json and inventory.json share. The
// repository files of the projects are read on the way, and a problem
// with one is added to problems.
func detect(collection *input.Collection, parsed map[string]bool, problems *[]string) detection {
	return detectWith(collection, parsed, projectRepositories(collection.Artifacts, problems))
}

// detectWith returns what detect --json and inventory.json share, given the
// repositories of the projects.
func detectWith(collection *input.Collection, parsed map[string]bool,
	repositories map[string]projectGit) detection {
	projects := projectGroups(collection, parsed)
	for i := range projects {
		if git, ok := repositories[projects[i].path]; ok {
			projects[i].git = git
		} else {
			projects[i].git = emptyGit()
		}
	}
	return detection{input: collection.Source, kind: collection.Kind, host: collection.Host,
		notes: collection.Notes, homes: homeAgents(collection, parsed), projects: projects}
}

// runDetect prints the homes, agents and projects of the input as tables
// or, with --json, as one JSON object.
func runDetect(ctx context.Context, options DetectOptions, env Environment, stdout,
	stderr io.Writer) int {
	found, err := matcher()
	if err != nil {
		return printError(ctx, stderr, err.Error())
	}
	out := output{secrets.NewRecorder(options.Secrets)}
	collection, workCreated, message := openInput(ctx, options.InputOptions, found, env, out.recorder)
	defer removeCreated(workCreated)
	if collection == nil {
		return printError(ctx, stderr, message)
	}
	defer closeInput(ctx, collection, options.KeepExtracted, stderr)
	parsed := parserAgents(env.Parsers())
	detected := detect(collection, parsed, &collection.Problems)
	if ctx.Err() != nil {
		return printError(ctx, stderr, "")
	}
	if options.JSON {
		_ = writeJSON(stdout, out.document(detectDocument(detected, collection, options.Files)))
		return ExitFound
	}
	return printDetect(stdout, out, detected, collection, parsed, options.Files)
}

// detectJSON is the document detect --json prints: the detection, the
// problems and, with --files, every artifact's user, agent and path.
type detectJSON struct {
	detectionJSON
	Problems []string    `json:"problems"`
	Files    *[]fileJSON `json:"files,omitempty"`
}

// fileJSON is one artifact of detect --json --files.
type fileJSON struct {
	User  string `json:"user"`
	Agent string `json:"agent"`
	Path  string `json:"path"`
}

// detectDocument returns the document detect --json prints.
func detectDocument(detected detection, collection *input.Collection, files bool) detectJSON {
	document := detectJSON{detectionJSON: newDetectionJSON(detected),
		Problems: list(collection.Problems)}
	if files {
		artifacts := make([]fileJSON, len(collection.Artifacts))
		for i, artifact := range collection.Artifacts {
			artifacts[i] = fileJSON{User: artifact.Home.User, Agent: artifact.Agent, Path: artifact.Original}
		}
		document.Files = &artifacts
	}
	return document
}

// printDetect prints the tables of detect and returns 1 when the input
// holds no agent files.
func printDetect(w io.Writer, out output, detected detection, collection *input.Collection,
	parsed map[string]bool, files bool) int {
	fmt.Fprint(w, "input:   "+out.shown(collection.Source)+"\n")
	kind := collection.Kind
	if kind == input.KindLoose {
		kind += " (no manifest; user and home inferred from paths)"
	}
	fmt.Fprint(w, "kind:    "+kind+"\n")
	fmt.Fprint(w, "host:    "+out.shown(orUnknown(collection.Host))+"\n")
	for _, note := range collection.Notes {
		fmt.Fprint(w, "note:    "+out.shown(note)+"\n")
	}
	for _, problem := range collection.Problems {
		fmt.Fprint(w, "problem: "+out.shown(problem)+"\n")
	}
	table := detectTable(collection, parsed)
	if len(table) == 0 && len(detected.projects) == 0 {
		fmt.Fprintln(w, "no agent artifacts found")
		return ExitNothing
	}
	printHomeTable(w, out, table)
	if len(detected.projects) > 0 {
		fmt.Fprintln(w)
		printProjectTable(w, out, detected.projects)
	}
	if files {
		fmt.Fprintln(w)
		for _, artifact := range collection.Artifacts {
			fmt.Fprint(w, out.shown(artifact.Home.User)+"\t"+out.shown(artifact.Agent)+"\t"+
				out.shown(artifact.Original)+"\n")
		}
	}
	return ExitFound
}

// orUnknown returns a host name, or "(unknown)" for none.
func orUnknown(host string) string {
	if host == "" {
		return "(unknown)"
	}
	return host
}

// tableRow is one row of the detect table: a user, home and agent.
type tableRow struct {
	user, original, home, agent string
	files, bytes                int64
	parser                      bool
}

// detectTable returns one row per user, home and agent of the homes that
// are not project directories, in the order each was first met.
func detectTable(collection *input.Collection, parsed map[string]bool) []tableRow {
	var rows firstMet[[3]string, tableRow]
	for _, artifact := range collection.Artifacts {
		home := artifact.Home
		if home.Project {
			continue
		}
		key := [3]string{home.User, home.Original, artifact.Agent}
		row := rows.entry(key, tableRow{user: home.User, original: home.Original, home: home.ListedPath(),
			agent: artifact.Agent, parser: parsed[artifact.Agent]})
		row.files++
		row.bytes = addBytes(row.bytes, artifactBytes(artifact))
	}
	return rows.values
}

// printHomeTable prints the table of homes and agents, if it has rows.
func printHomeTable(w io.Writer, out output, rows []tableRow) {
	if len(rows) == 0 {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%-14s %-32s %-18s %8s %12s  %s\n", "user", "home", "agent", "files", "bytes",
		"parser")
	for _, row := range rows {
		user := row.user
		if user == "" {
			user = "?"
		}
		fmt.Fprintf(w, "%-14s %-32.32s %-18s %8d %12d  %s\n", out.shown(user), out.shown(row.home),
			out.shown(row.agent), row.files, row.bytes, parserLabel(row.parser))
	}
}

// parserLabel returns the parser column of a table.
func parserLabel(parser bool) string {
	if parser {
		return "yes"
	}
	return "detect only"
}

// printProjectTable prints one line per agent of each project, a (shared)
// line for its shared files and a remotes: line when its repository has
// remotes; the path and users stand on the project's first line only.
func printProjectTable(w io.Writer, out output, projects []projectEntry) {
	printProjectLine(w, "project", "users", "agent", "files", "bytes", "parser")
	for _, project := range projects {
		first := true
		line := func(agent, files, size, parser string) {
			path, users := "", ""
			if first {
				path = fmt.Sprintf("%.40s", out.shown(project.path))
				users = strings.Join(out.shownAll(project.users), ",")
				if users == "" {
					users = "?"
				}
				first = false
			}
			printProjectLine(w, path, users, agent, files, size, parser)
		}
		for _, agent := range project.agents {
			line(out.shown(agent.agent), fmt.Sprint(agent.files), fmt.Sprint(agent.bytes),
				parserLabel(agent.parser))
		}
		if project.sharedFiles > 0 {
			line("(shared)", fmt.Sprint(project.sharedFiles), fmt.Sprint(project.sharedBytes), "")
		}
		if len(project.git.remotes) > 0 {
			fmt.Fprint(w, "  remotes: "+out.shown(remotesText(project.git.remotes))+"\n")
		}
	}
}

// printProjectLine prints one line of the project table, trailing
// whitespace removed.
func printProjectLine(w io.Writer, path, users, agent, files, size, parser string) {
	line := fmt.Sprintf("%-40s %-14s %-18s %8s %12s  %s", path, users, agent, files, size, parser)
	fmt.Fprintln(w, strings.TrimRightFunc(line, unicode.IsSpace))
}

// remotesText returns the remotes as "<name> <url> ... (push <pushurl> ...)"
// joined with ", ".
func remotesText(remotes []gitmeta.Remote) string {
	parts := make([]string, len(remotes))
	for i, remote := range remotes {
		parts[i] = strings.Join(append([]string{remote.Name}, remote.URLs...), " ")
		if len(remote.PushURLs) > 0 {
			parts[i] += " (push " + strings.Join(remote.PushURLs, " ") + ")"
		}
	}
	return strings.Join(parts, ", ")
}
