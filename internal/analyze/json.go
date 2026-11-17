package analyze

import (
	"github.com/seanthegeek/doubleagent/internal/gitmeta"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// member returns one key and value of an object.
func member(key string, value jsonvalue.Value) jsonvalue.Member {
	return jsonvalue.Member{Key: key, Value: value}
}

// stringArray returns texts as a JSON array of strings.
func stringArray(texts []string) jsonvalue.Value {
	items := make([]jsonvalue.Value, len(texts))
	for i, text := range texts {
		items[i] = jsonvalue.NewString(text)
	}
	return jsonvalue.NewArray(items...)
}

// detectionMembers returns the keys detect --json and inventory.json share,
// in order: input, kind, host, notes, homes and projects.
func detectionMembers(detected detection) []jsonvalue.Member {
	homes := make([]jsonvalue.Value, len(detected.homes))
	for i, home := range detected.homes {
		homes[i] = jsonvalue.NewObject(homeMembers(home, nil)...)
	}
	projects := make([]jsonvalue.Value, len(detected.projects))
	for i, project := range detected.projects {
		projects[i] = projectJSON(project)
	}
	return []jsonvalue.Member{
		member("input", jsonvalue.NewString(detected.input)),
		member("kind", jsonvalue.NewString(detected.kind)),
		member("host", jsonvalue.NewString(detected.host)),
		member("notes", stringArray(detected.notes)),
		member("homes", jsonvalue.NewArray(homes...)),
		member("projects", jsonvalue.NewArray(projects...)),
	}
}

// homeMembers returns the keys of one home: user, home, inferred and its
// agents, each agent's object ending with the members extra gives it.
func homeMembers(home homeEntry, extra func(agent string) []jsonvalue.Member) []jsonvalue.Member {
	agents := make([]jsonvalue.Value, len(home.agents))
	for i, agent := range home.agents {
		members := agentMembers(agent)
		if extra != nil {
			members = append(members, extra(agent.agent)...)
		}
		agents[i] = jsonvalue.NewObject(members...)
	}
	return []jsonvalue.Member{
		member("user", jsonvalue.NewString(home.user)),
		member("home", jsonvalue.NewString(home.home)),
		member("inferred", jsonvalue.NewBool(home.inferred)),
		member("agents", jsonvalue.NewArray(agents...)),
	}
}

// agentMembers returns the keys of one agent of a home or project: agent,
// files, bytes and parser.
func agentMembers(agent agentFiles) []jsonvalue.Member {
	return []jsonvalue.Member{
		member("agent", jsonvalue.NewString(agent.agent)),
		member("files", jsonvalue.NewInt(agent.files)),
		member("bytes", jsonvalue.NewInt(agent.bytes)),
		member("parser", jsonvalue.NewBool(agent.parser)),
	}
}

// projectJSON returns one project of detect --json: path, users, agents,
// shared_files, shared_bytes, remotes and head.
func projectJSON(project projectEntry) jsonvalue.Value {
	agents := make([]jsonvalue.Value, len(project.agents))
	for i, agent := range project.agents {
		agents[i] = jsonvalue.NewObject(agentMembers(agent)...)
	}
	return jsonvalue.NewObject(
		member("path", jsonvalue.NewString(project.path)),
		member("users", stringArray(project.users)),
		member("agents", jsonvalue.NewArray(agents...)),
		member("shared_files", jsonvalue.NewInt(project.sharedFiles)),
		member("shared_bytes", jsonvalue.NewInt(project.sharedBytes)),
		member("remotes", remotesJSON(project.git.remotes)),
		member("head", jsonvalue.NewString(project.git.head)),
	)
}

// remotesJSON returns remotes as a JSON array of objects with name, urls and
// pushurls.
func remotesJSON(remotes []gitmeta.Remote) jsonvalue.Value {
	items := make([]jsonvalue.Value, len(remotes))
	for i, remote := range remotes {
		items[i] = jsonvalue.NewObject(
			member("name", jsonvalue.NewString(remote.Name)),
			member("urls", stringArray(remote.URLs)),
			member("pushurls", stringArray(remote.PushURLs)),
		)
	}
	return jsonvalue.NewArray(items...)
}
