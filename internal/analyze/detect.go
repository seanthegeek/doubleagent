package analyze

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/gitmeta"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// fileCounts is the number and total size of one agent's files.
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

// countAgent adds one artifact to the counts of its agent in list, kept in
// the order the agents were first met.
func countAgent(list []fileCounts, agent string, size int64) []fileCounts {
	index := slices.IndexFunc(list, func(each fileCounts) bool { return each.agent == agent })
	if index < 0 {
		list = append(list, fileCounts{agent: agent})
		index = len(list) - 1
	}
	list[index].files++
	list[index].bytes += size
	return list
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
	counts := map[*input.Home][]fileCounts{}
	for _, artifact := range collection.Artifacts {
		if !artifact.Home.Project {
			counts[artifact.Home] = countAgent(counts[artifact.Home], artifact.Agent,
				artifactBytes(artifact))
		}
	}
	var homes []homeEntry
	for _, home := range collection.Homes {
		if !home.Project {
			homes = append(homes, homeEntry{user: home.User, home: home.Original,
				inferred: home.Inferred, agents: withParsers(counts[home], parsed)})
		}
	}
	return homes
}

// projectGroups returns the project directories of the input sorted by
// path: their users, their agents' files and their shared files.
func projectGroups(collection *input.Collection, parsed map[string]bool) []projectEntry {
	groups := map[string]*projectEntry{}
	counts := map[string][]fileCounts{}
	for _, artifact := range collection.Artifacts {
		if !artifact.Home.Project {
			continue
		}
		path := homePath(artifact.Home)
		group, ok := groups[path]
		if !ok {
			group = &projectEntry{path: path, users: []string{}}
			groups[path] = group
		}
		if user := artifact.Home.User; user != "" && !slices.Contains(group.users, user) {
			group.users = append(group.users, user)
		}
		size := artifactBytes(artifact)
		if artifact.Agent == input.ProjectAgent {
			group.sharedFiles++
			group.sharedBytes += size
		} else {
			counts[path] = countAgent(counts[path], artifact.Agent, size)
		}
	}
	projects := make([]projectEntry, 0, len(groups))
	for _, path := range sortedKeys(groups) {
		group := groups[path]
		slices.Sort(group.users)
		group.agents = withParsers(counts[path], parsed)
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
	repositories := projectRepositories(collection.Artifacts, problems)
	projects := projectGroups(collection, parsed)
	for i := range projects {
		if git, ok := repositories[projects[i].path]; ok {
			projects[i].git = git
		} else {
			projects[i].git = projectGit{remotes: []gitmeta.Remote{}}
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
	collection, message := openInput(options.InputOptions, found)
	if collection == nil {
		return printError(ctx, stderr, message)
	}
	defer closeInput(collection, options.KeepExtracted, stderr)
	parsed := parserAgents(env.Parsers())
	detected := detect(collection, parsed, &collection.Problems)
	if options.JSON {
		_, _ = io.WriteString(stdout, detectJSON(detected, collection, options.Files)+"\n")
		return ExitFound
	}
	return printDetect(stdout, detected, collection, parsed, options.Files)
}

// detectJSON returns the object detect --json prints: the detection, the
// problems and, with files, every artifact's user, agent and path.
func detectJSON(detected detection, collection *input.Collection, files bool) string {
	members := append(detectionMembers(detected), pyjson.Member{Key: "problems",
		Value: stringArray(collection.Problems)})
	if files {
		list := make([]pyjson.Value, len(collection.Artifacts))
		for i, artifact := range collection.Artifacts {
			list[i] = pyjson.NewObject(
				pyjson.Member{Key: "user", Value: pyjson.NewString(artifact.Home.User)},
				pyjson.Member{Key: "agent", Value: pyjson.NewString(artifact.Agent)},
				pyjson.Member{Key: "path", Value: pyjson.NewString(artifact.Original)})
		}
		members = append(members, pyjson.Member{Key: "files", Value: pyjson.NewArray(list...)})
	}
	return pyjson.Indented(pyjson.NewObject(members...))
}

// printDetect prints the tables of detect and returns 1 when the input
// holds no agent files.
func printDetect(w io.Writer, detected detection, collection *input.Collection,
	parsed map[string]bool, files bool) int {
	fmt.Fprintf(w, "input:   %s\n", collection.Source)
	kind := collection.Kind
	if kind == input.KindLoose {
		kind += " (no manifest; user and home inferred from paths)"
	}
	fmt.Fprintf(w, "kind:    %s\n", kind)
	fmt.Fprintf(w, "host:    %s\n", orUnknown(collection.Host))
	for _, note := range collection.Notes {
		fmt.Fprintf(w, "note:    %s\n", note)
	}
	for _, problem := range collection.Problems {
		fmt.Fprintf(w, "problem: %s\n", problem)
	}
	table := detectTable(collection, parsed)
	if len(table) == 0 && len(detected.projects) == 0 {
		fmt.Fprintln(w, "no agent artifacts found")
		return ExitNothing
	}
	printHomeTable(w, table)
	if len(detected.projects) > 0 {
		fmt.Fprintln(w)
		printProjectTable(w, detected.projects)
	}
	if files {
		fmt.Fprintln(w)
		for _, artifact := range collection.Artifacts {
			fmt.Fprintf(w, "%s\t%s\t%s\n", artifact.Home.User, artifact.Agent, artifact.Original)
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
	var rows []tableRow
	for _, artifact := range collection.Artifacts {
		home := artifact.Home
		if home.Project {
			continue
		}
		index := slices.IndexFunc(rows, func(row tableRow) bool {
			return row.user == home.User && row.original == home.Original && row.agent == artifact.Agent
		})
		if index < 0 {
			rows = append(rows, tableRow{user: home.User, original: home.Original,
				home: homePath(home), agent: artifact.Agent, parser: parsed[artifact.Agent]})
			index = len(rows) - 1
		}
		rows[index].files++
		rows[index].bytes += artifactBytes(artifact)
	}
	return rows
}

// printHomeTable prints the table of homes and agents, if it has rows.
func printHomeTable(w io.Writer, rows []tableRow) {
	if len(rows) == 0 {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s %s %s %8s %12s  %s\n", pytext.PadRight("user", 14),
		pytext.PadRight("home", 32), pytext.PadRight("agent", 18), "files", "bytes", "parser")
	for _, row := range rows {
		user := row.user
		if user == "" {
			user = "?"
		}
		fmt.Fprintf(w, "%s %s %s %8d %12d  %s\n", pytext.PadRight(user, 14),
			pytext.PadRight(pytext.SliceCodePoints(row.home, 32), 32),
			pytext.PadRight(row.agent, 18), row.files, row.bytes, parserLabel(row.parser))
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
func printProjectTable(w io.Writer, projects []projectEntry) {
	printProjectLine(w, "project", "users", "agent", "files", "bytes", "parser")
	for _, project := range projects {
		first := true
		line := func(agent, files, size, parser string) {
			path, users := "", ""
			if first {
				path = pytext.SliceCodePoints(project.path, 40)
				users = strings.Join(project.users, ",")
				if users == "" {
					users = "?"
				}
				first = false
			}
			printProjectLine(w, path, users, agent, files, size, parser)
		}
		for _, agent := range project.agents {
			line(agent.agent, fmt.Sprint(agent.files), fmt.Sprint(agent.bytes),
				parserLabel(agent.parser))
		}
		if project.sharedFiles > 0 {
			line("(shared)", fmt.Sprint(project.sharedFiles), fmt.Sprint(project.sharedBytes), "")
		}
		if len(project.git.remotes) > 0 {
			fmt.Fprintf(w, "  remotes: %s\n", remotesText(project.git.remotes))
		}
	}
}

// printProjectLine prints one line of the project table, trailing
// whitespace removed.
func printProjectLine(w io.Writer, path, users, agent, files, size, parser string) {
	line := fmt.Sprintf("%s %s %s %s %s  %s", pytext.PadRight(path, 40), pytext.PadRight(users, 14),
		pytext.PadRight(agent, 18), pytext.PadLeft(files, 8), pytext.PadLeft(size, 12), parser)
	fmt.Fprintln(w, pytext.StripRight(line))
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
