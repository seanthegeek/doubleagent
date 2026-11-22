package analyze

import (
	"encoding/json"
	"io"

	"github.com/seanthegeek/doubleagent/internal/gitmeta"
)

// writeJSON writes document as indented JSON (two spaces), UTF-8 kept and
// HTML characters not escaped, ending in a newline.
func writeJSON(w io.Writer, document any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}

// list returns items, or an empty list for none, so that encoding/json
// writes [] rather than null.
func list[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// detectionJSON holds the keys detect --json and inventory.json share, in
// order: input, kind, host, notes, homes and projects.
type detectionJSON struct {
	Input    string        `json:"input"`
	Kind     string        `json:"kind"`
	Host     string        `json:"host"`
	Notes    []string      `json:"notes"`
	Homes    []homeJSON    `json:"homes"`
	Projects []projectJSON `json:"projects"`
}

// homeJSON is one home: user, home, inferred and its agents.
type homeJSON struct {
	User     string      `json:"user"`
	Home     string      `json:"home"`
	Inferred bool        `json:"inferred"`
	Agents   []agentJSON `json:"agents"`
}

// agentJSON is one agent of a home or project: agent, files, bytes and
// parser. Bytes, like shared_bytes, is held at the largest int64 when the
// sizes add up to more.
type agentJSON struct {
	Agent  string `json:"agent"`
	Files  int64  `json:"files"`
	Bytes  int64  `json:"bytes"`
	Parser bool   `json:"parser"`
}

// projectJSON is one project of detect --json: path, users, agents,
// shared_files, shared_bytes, remotes and head.
type projectJSON struct {
	Path        string       `json:"path"`
	Users       []string     `json:"users"`
	Agents      []agentJSON  `json:"agents"`
	SharedFiles int64        `json:"shared_files"`
	SharedBytes int64        `json:"shared_bytes"`
	Remotes     []remoteJSON `json:"remotes"`
	Head        string       `json:"head"`
}

// remoteJSON is one git remote: name, urls and pushurls.
type remoteJSON struct {
	Name     string   `json:"name"`
	URLs     []string `json:"urls"`
	PushURLs []string `json:"pushurls"`
}

// newDetectionJSON returns the shared keys of a detection.
func newDetectionJSON(detected detection) detectionJSON {
	homes := make([]homeJSON, len(detected.homes))
	for i, home := range detected.homes {
		homes[i] = newHomeJSON(home)
	}
	projects := make([]projectJSON, len(detected.projects))
	for i, project := range detected.projects {
		projects[i] = projectJSON{Path: project.path, Users: list(project.users),
			Agents: agentsJSON(project.agents), SharedFiles: project.sharedFiles,
			SharedBytes: project.sharedBytes, Remotes: remotesJSON(project.git.remotes),
			Head: project.git.head}
	}
	return detectionJSON{Input: detected.input, Kind: detected.kind, Host: detected.host,
		Notes: list(detected.notes), Homes: homes, Projects: projects}
}

// newHomeJSON returns one home with its agents.
func newHomeJSON(home homeEntry) homeJSON {
	return homeJSON{User: home.user, Home: home.home, Inferred: home.inferred,
		Agents: agentsJSON(home.agents)}
}

// agentsJSON returns agents in the form of a home's or project's agents.
func agentsJSON(agents []agentFiles) []agentJSON {
	out := make([]agentJSON, len(agents))
	for i, agent := range agents {
		out[i] = agentJSON{Agent: agent.agent, Files: agent.files, Bytes: agent.bytes,
			Parser: agent.parser}
	}
	return out
}

// remotesJSON returns remotes with their urls and pushurls.
func remotesJSON(remotes []gitmeta.Remote) []remoteJSON {
	out := make([]remoteJSON, len(remotes))
	for i, remote := range remotes {
		out[i] = remoteJSON{Name: remote.Name, URLs: list(remote.URLs),
			PushURLs: list(remote.PushURLs)}
	}
	return out
}
