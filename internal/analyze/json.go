package analyze

import (
	"github.com/seanthegeek/doubleagent/internal/gitmeta"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// member returns one key and value of an object.
func member(key string, value pyjson.Value) pyjson.Member {
	return pyjson.Member{Key: key, Value: value}
}

// stringArray returns texts as a JSON array of strings.
func stringArray(texts []string) pyjson.Value {
	items := make([]pyjson.Value, len(texts))
	for i, text := range texts {
		items[i] = pyjson.NewString(text)
	}
	return pyjson.NewArray(items...)
}

// detectionMembers returns the keys detect --json and inventory.json share,
// in order: input, kind, host, notes, homes and projects.
func detectionMembers(detected detection) []pyjson.Member {
	homes := make([]pyjson.Value, len(detected.homes))
	for i, home := range detected.homes {
		homes[i] = pyjson.NewObject(homeMembers(home, nil)...)
	}
	projects := make([]pyjson.Value, len(detected.projects))
	for i, project := range detected.projects {
		projects[i] = projectJSON(project)
	}
	return []pyjson.Member{
		member("input", pyjson.NewString(detected.input)),
		member("kind", pyjson.NewString(detected.kind)),
		member("host", pyjson.NewString(detected.host)),
		member("notes", stringArray(detected.notes)),
		member("homes", pyjson.NewArray(homes...)),
		member("projects", pyjson.NewArray(projects...)),
	}
}

// homeMembers returns the keys of one home: user, home, inferred and its
// agents, each agent's object ending with the members extra gives it.
func homeMembers(home homeEntry, extra func(agent string) []pyjson.Member) []pyjson.Member {
	agents := make([]pyjson.Value, len(home.agents))
	for i, agent := range home.agents {
		members := agentMembers(agent)
		if extra != nil {
			members = append(members, extra(agent.agent)...)
		}
		agents[i] = pyjson.NewObject(members...)
	}
	return []pyjson.Member{
		member("user", pyjson.NewString(home.user)),
		member("home", pyjson.NewString(home.home)),
		member("inferred", pyjson.NewBool(home.inferred)),
		member("agents", pyjson.NewArray(agents...)),
	}
}

// agentMembers returns the keys of one agent of a home or project: agent,
// files, bytes and parser.
func agentMembers(agent agentFiles) []pyjson.Member {
	return []pyjson.Member{
		member("agent", pyjson.NewString(agent.agent)),
		member("files", pyjson.NewInt(agent.files)),
		member("bytes", pyjson.NewInt(agent.bytes)),
		member("parser", pyjson.NewBool(agent.parser)),
	}
}

// projectJSON returns one project of detect --json: path, users, agents,
// shared_files, shared_bytes, remotes and head.
func projectJSON(project projectEntry) pyjson.Value {
	agents := make([]pyjson.Value, len(project.agents))
	for i, agent := range project.agents {
		agents[i] = pyjson.NewObject(agentMembers(agent)...)
	}
	return pyjson.NewObject(
		member("path", pyjson.NewString(project.path)),
		member("users", stringArray(project.users)),
		member("agents", pyjson.NewArray(agents...)),
		member("shared_files", pyjson.NewInt(project.sharedFiles)),
		member("shared_bytes", pyjson.NewInt(project.sharedBytes)),
		member("remotes", remotesJSON(project.git.remotes)),
		member("head", pyjson.NewString(project.git.head)),
	)
}

// remotesJSON returns remotes as a JSON array of objects with name, urls and
// pushurls.
func remotesJSON(remotes []gitmeta.Remote) pyjson.Value {
	items := make([]pyjson.Value, len(remotes))
	for i, remote := range remotes {
		items[i] = pyjson.NewObject(
			member("name", pyjson.NewString(remote.Name)),
			member("urls", stringArray(remote.URLs)),
			member("pushurls", stringArray(remote.PushURLs)),
		)
	}
	return pyjson.NewArray(items...)
}
