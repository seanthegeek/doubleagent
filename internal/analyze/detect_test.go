package analyze

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// detectFiles is a collection of two homes and a project with a repository;
// the expected outputs below are detect's over these files.
var detectFiles = []collectedFile{
	{"alice", "/home/alice", "codex-cli", ".codex/sessions/a.jsonl", "x\n"},
	{"alice", "/home/alice", "claude-code", ".claude/settings.json", "{}\n"},
	{"bob", "/home/bob", "aider", ".aider.chat.history.md", "# hi é\n"},
	{"alice", "/srv/proj", "project", ".git/config",
		"[remote \"origin\"]\n\turl = git@example.com:a/p.git\n"},
	{"alice", "/srv/proj", "project", ".git/HEAD", "ref: refs/heads/main\n"},
	{"alice", "/srv/proj", "claude-code", ".claude/settings.local.json", "{}\n"},
	{"bob", "/srv/proj", "project", "AGENTS.md", "# x\n"},
}

const detectText = `input:   {ROOT}/col
kind:    collected
host:    h1

user           home                             agent                 files        bytes  parser
alice          /home/alice                      codex-cli                 1            2  yes
alice          /home/alice                      claude-code               1            3  yes
bob            /home/bob                        aider                     1            8  yes

project                                  users          agent                 files        bytes  parser
/srv/proj                                alice,bob      claude-code               1            3  yes
                                                        (shared)                  3           74
  remotes: origin git@example.com:a/p.git
`

const detectFilesText = detectText + `
alice	codex-cli	/home/alice/.codex/sessions/a.jsonl
alice	claude-code	/home/alice/.claude/settings.json
bob	aider	/home/bob/.aider.chat.history.md
alice	project	/srv/proj/.git/config
alice	project	/srv/proj/.git/HEAD
alice	claude-code	/srv/proj/.claude/settings.local.json
bob	project	/srv/proj/AGENTS.md
`

const detectJSONText = "{\n  \"input\": \"{ROOT}/col\",\n  \"kind\": \"collected\",\n  \"host\": \"h1\",\n  \"notes\": [],\n  \"homes\": [\n    {\n      \"user\": \"alice\",\n      \"home\": \"/home/alice\",\n      \"inferred\": false,\n      \"agents\": [\n        {\n          \"agent\": \"claude-code\",\n          \"files\": 1,\n          \"bytes\": 3,\n          \"parser\": true\n        },\n        {\n          \"agent\": \"codex-cli\",\n          \"files\": 1,\n          \"bytes\": 2,\n          \"parser\": true\n        }\n      ]\n    },\n    {\n      \"user\": \"bob\",\n      \"home\": \"/home/bob\",\n      \"inferred\": false,\n      \"agents\": [\n        {\n          \"agent\": \"aider\",\n          \"files\": 1,\n          \"bytes\": 8,\n          \"parser\": true\n        }\n      ]\n    }\n  ],\n  \"projects\": [\n    {\n      \"path\": \"/srv/proj\",\n      \"users\": [\n        \"alice\",\n        \"bob\"\n      ],\n      \"agents\": [\n        {\n          \"agent\": \"claude-code\",\n          \"files\": 1,\n          \"bytes\": 3,\n          \"parser\": true\n        }\n      ],\n      \"shared_files\": 3,\n      \"shared_bytes\": 74,\n      \"remotes\": [\n        {\n          \"name\": \"origin\",\n          \"urls\": [\n            \"git@example.com:a/p.git\"\n          ],\n          \"pushurls\": []\n        }\n      ],\n      \"head\": \"main\"\n    }\n  ],\n  \"problems\": []\n}\n"

func TestDetectReportsEachHomeAndProject(t *testing.T) {
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"), []jsonvalue.Member{
		member("hostname", jsonvalue.NewString("h1")),
		member("projects", stringArray([]string{"/srv/proj"}))}, detectFiles)
	if err := osfile.MkdirAll(filepath.Join(work, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := testEnvironment(fakeParsers("claude-code", "codex-cli", "aider")...)
	tests := []struct {
		name           string
		args           []string
		code           int
		stdout, stderr string
	}{
		{"the tables", []string{"detect", collection}, ExitFound, detectText, ""},
		{"the tables and every file", []string{"detect", "--files", collection}, ExitFound,
			detectFilesText, ""},
		{"one JSON object", []string{"detect", "--json", collection}, ExitFound, detectJSONText, ""},
		{"an input without agent files exits 1", []string{"detect", filepath.Join(work, "empty")},
			ExitNothing, "input:   {ROOT}/empty\nkind:    loose (no manifest; user and home " +
				"inferred from paths)\nhost:    (unknown)\nno agent artifacts found\n", ""},
		{"a missing input is an error", []string{"detect", filepath.Join(work, "nope")}, ExitError,
			"", "error: {ROOT}/nope: no such file or directory\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, stdout, stderr := runCommand(env, test.args...)
			stdout, stderr = showRoot(stdout, work), showRoot(stderr, work)
			if code != test.code || stdout != test.stdout || stderr != test.stderr {
				t.Errorf("exit %d, stdout:\n%s\nstderr: %q\nwant exit %d, stdout:\n%s\nstderr: %q",
					code, stdout, stderr, test.code, test.stdout, test.stderr)
			}
		})
	}
}
