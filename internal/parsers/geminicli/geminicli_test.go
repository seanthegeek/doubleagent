package geminicli

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// The rows each file gives are checked by the golden output of the analysis
// commands (internal/analyze), over inputs that hold every parsed agent;
// these tests cover the helpers on their own.

func TestTheHistoryObjectStartTakesJSONWhitespaceOnly(t *testing.T) {
	if !historyObjectPattern.MatchString(" \r\n{\t\"history\" : ") {
		t.Error("JSON whitespace around the key is not taken")
	}
	for _, space := range []string{" ", " ", "\u0085"} {
		for _, text := range []string{space + `{"history": `, `{"history"` + space + `: `} {
			if historyObjectPattern.MatchString(text) {
				t.Errorf("%q matches", text)
			}
		}
	}
}

func TestATagCheckpointIsNotAToolCheckpointNorTheOtherWay(t *testing.T) {
	checkpoint := ".gemini/tmp/proj/checkpoint-my%20tag.json"
	tool := ".gemini/tmp/proj/checkpoints/2026-10-01T09-00-21_500Z-util.ts-replace.json"
	if toolCheckpointPattern.MatchString(checkpoint) || checkpointPattern.MatchString(tool) {
		t.Error("a checkpoint pattern matches the other kind")
	}
	if !checkpointPattern.MatchString(checkpoint) || !toolCheckpointPattern.MatchString(tool) {
		t.Error("a checkpoint pattern misses its own kind")
	}
}

// contextKindCases are texts and the context kinds they are read as:
// the CLI's own openings, then the person's texts with a tag or phrase
// typed inside them.
func contextKindCases() []struct{ text, kind string } {
	return []struct{ text, kind string }{
		{"<session_context>\nThis is the Gemini CLI. We are setting up the context for our chat.\n" +
			"My operating system is: linux\n</session_context>", "session_context"},
		{"  \n<hook_context>Branch main is protected.</hook_context>", "hook_context"},
		{"<state_snapshot>\n<overall_goal>x</overall_goal>\n</state_snapshot>", "state_snapshot"},
		{"<scratchpad>The user listed src and read util.ts.</scratchpad>\n<state_snapshot>\n    " +
			"<overall_goal>Review util.ts.</overall_goal>\n</state_snapshot>", "state_snapshot"},
		{"User has manually exited Plan Mode. Switching to Default mode (edits will require " +
			"confirmation).", "plan_mode_exit"},
		{"- **Workspace Directories:**\n  - /srv/proj\n- **Directory Structure:**\n", "directory_context"},
		{"Here is the user's editor context as a JSON object. This is for your information only.\n" +
			"```json\n{\"activeFile\": {\"path\": \"/srv/proj/src/util.ts\"}}\n```", "ide_context"},
		{"Here is a summary of changes in the user's editor context, in JSON format. This is for " +
			"your information only.\n```json\n{\"changes\": {}}\n```", "ide_context"},
		{"<scratchpad>notes, no snapshot</scratchpad>", ""},
		{"please explain <session_context> to me", ""},
		{"Plan approved. Switching to Default mode (edits will require confirmation).", ""},
		{"user has manually exited plan mode. switching to default", ""},
		{"Workspace Directories: /srv/proj", ""},
		{"Here is the user's editor context", ""},
		{"I ran the following shell command:\n```sh\nls\n```\n\nThis produced the following " +
			"result:\n```\nsrc\n```", "shell_command"},
		{"User steering update:\n<user_input>\nskip the tests\n</user_input>\nInternal instruction: " +
			"Re-evaluate the active plan using this user steering update.", "steering_update"},
		{"Background execution update:\n<background_output>\nbuild done\n</background_output>",
			"background_update"},
		{"User hints:\n<user_input>\n- check the lockfile\n</user_input>\n\nInternal instruction: " +
			"Re-evaluate the active plan using this user steering update.", "user_hints"},
		{"\n<loaded_context>\n<project_context>\nTests run with pnpm.\n</project_context>\n" +
			"</loaded_context>", "loaded_context"},
		{"loaded_context: the notes I wrote", ""},
		{"user hints: lowercase is typed", ""},
		{"Hints: none", ""},
		{"Please continue.", ""},
		{"/help", ""},
		{"? what changed", ""},
		{"", ""},
	}
}

func TestContextKindOfEachOpening(t *testing.T) {
	for _, test := range contextKindCases() {
		t.Run(test.text, func(t *testing.T) {
			if got := contextKind(test.text); got != test.kind {
				t.Errorf("got %q, want %q", got, test.kind)
			}
		})
	}
}

func TestNameTimeReadsTheTimeOfAFileName(t *testing.T) {
	tests := []struct{ name, want string }{
		{"2026-10-01T09-00-00_123Z", "2026-10-01T09:00:00.123Z"},
		{"2026-10-01T09-00-00-123Z", "2026-10-01T09:00:00.123Z"},
		{"not-a-time", ""},
		{"", ""},
		{"2026-13-01T09-00-00_123Z", ""},
		{"2026-10-01T25-00-00_123Z", ""},
	}
	for _, test := range tests {
		if got := nameTime(test.name); got != test.want {
			t.Errorf("nameTime(%q) = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestUnquoteDecodesATagsEscapes(t *testing.T) {
	tests := []struct{ name, text, want string }{
		{"an escaped space", "a%20b", "a b"},
		{"an escaped character", "%E2%82%AC", "€"},
		{"each invalid byte becomes U+FFFD", "%e2%82", "\uFFFD\uFFFD"},
		{"text beside escapes is kept", "%41é%42", "AéB"},
		{"an escaped slash", "%C3%A9x%2F", "éx/"},
		{"a plus sign stays", "a+b", "a+b"},
		{"plain text stays", "plain", "plain"},
		{"a bare percent sign is shown as written", "100%", "100%"},
		{"an invalid escape is shown as written", "%zz%41", "%zz%41"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := unquote(test.text); got != test.want {
				t.Errorf("unquote(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}
}

func TestProjectFallsBackFromTheRootFileToProjectsJSONToTheHash(t *testing.T) {
	base := t.TempDir()
	write(t, filepath.Join(base, "tmp", "proj", ".project_root"), " /srv/proj\n")
	write(t, filepath.Join(base, "projects.json"), `{"projects": {"/srv/proj": "proj"}}`)
	hash := "6b9af143446a411aefa3edb76e4693799bd7513e7ed8d919962d6713d3d3b972"
	var problems parserkit.Problems
	steps := []struct {
		name, slug, hash, want string
		before                 func()
	}{
		{"the root file", "proj", "", "/srv/proj", nil},
		{"the projects.json slug", "proj", "", "/srv/proj", func() {
			if err := osfile.Remove(filepath.Join(base, "tmp", "proj", ".project_root")); err != nil {
				t.Fatal(err)
			}
		}},
		{"the projects.json key of the hash", "other", hash, "/srv/proj", nil},
		{"the projects.json key of a hash slug", hash, "", "/srv/proj", nil},
		{"the hash itself", "other", strings.Repeat("f", 64), strings.Repeat("f", 64), nil},
		{"nothing", "other", "", "", nil},
	}
	subject := New()
	for _, step := range steps {
		if step.before != nil {
			step.before()
		}
		if got := subject.resolveProject(nil, func(names ...string) string {
			return filepath.Join(append([]string{base}, names...)...)
		}, step.slug, step.hash, &problems); got != step.want {
			t.Errorf("%s: got %q, want %q", step.name, got, step.want)
		}
	}
	if len(problems) != 0 {
		t.Errorf("problems %+v", problems)
	}
}

func TestOnlyTheHomesHookAndAgentFilesAreWanted(t *testing.T) {
	subject := New()
	for _, rel := range []string{".gemini/trusted_hooks.json", ".gemini/acknowledgments/agents.json"} {
		home := &input.Home{}
		if !subject.Wants(&input.Artifact{Rel: rel, Home: home}) {
			t.Errorf("%s of a home is not wanted", rel)
		}
		home.Project = true
		if subject.Wants(&input.Artifact{Rel: rel, Home: home}) {
			t.Errorf("%s of a project is wanted", rel)
		}
	}
}

// write writes text to path, making its directory.
func write(t *testing.T, path, text string) {
	t.Helper()
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Every file of a runtime directory looks its project up in one
// projects.json, which is decoded once per state of the file.
func TestProjectsJSONIsReadOncePerStateOfTheFile(t *testing.T) {
	const lookups = 300
	base := t.TempDir()
	projects := filepath.Join(base, "projects.json")
	write(t, projects, `{"projects": {"/srv/a": "a", "/srv/b": "b"}}`)
	locate := func(names ...string) string { return filepath.Join(append([]string{base}, names...)...) }
	subject := New()
	subject.Work = parserkit.CountingWork()
	var problems parserkit.Problems
	for i := range lookups {
		slug, want := "a", "/srv/a"
		if i%2 == 1 {
			slug, want = "b", "/srv/b"
		}
		if got := subject.resolveProject(nil, locate, slug, "", &problems); got != want {
			t.Fatalf("lookup %d gave %q, want %q", i, got, want)
		}
	}
	if reads := subject.Work.Count("projects.json reads"); reads != 1 {
		t.Errorf("projects.json decoded %d times for %d lookups, want once", reads, lookups)
	}
	write(t, projects, `{"projects": {"/srv/a2": "a", "/srv/b": "b"}}`)
	if got := subject.resolveProject(nil, locate, "a", "", &problems); got != "/srv/a2" {
		t.Errorf("a changed file gave %q, want the new key", got)
	}
	if reads := subject.Work.Count("projects.json reads"); reads != 2 || len(problems) != 0 {
		t.Errorf("%d reads and problems %+v after a change, want 2 and none", reads, problems)
	}
}

// Every file of a runtime directory reads its .project_root, which is read
// once per state of the file.
func TestTheProjectRootIsReadOncePerStateOfTheFile(t *testing.T) {
	const lookups = 300
	base := t.TempDir()
	root := filepath.Join(base, "tmp", "slug", ".project_root")
	if err := osfile.MkdirAll(filepath.Dir(root), 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, root, "/srv/a\n")
	locate := func(names ...string) string { return filepath.Join(append([]string{base}, names...)...) }
	subject := New()
	subject.Work = parserkit.CountingWork()
	var problems parserkit.Problems
	for i := range lookups {
		if got := subject.resolveProject(nil, locate, "slug", "", &problems); got != "/srv/a" {
			t.Fatalf("lookup %d gave %q, want /srv/a", i, got)
		}
	}
	if reads := subject.Work.Count(".project_root reads"); reads != 1 {
		t.Errorf(".project_root read %d times for %d lookups, want once", reads, lookups)
	}
	write(t, root, "/srv/longer\n")
	if got := subject.resolveProject(nil, locate, "slug", "", &problems); got != "/srv/longer" {
		t.Errorf("a changed file gave %q, want its new text", got)
	}
	if reads := subject.Work.Count(".project_root reads"); reads != 2 || len(problems) != 0 {
		t.Errorf("%d reads and problems %+v after a change, want 2 and none", reads, problems)
	}
}

// The sha256 of a key matches a hash slug or the session's project hash,
// the first such key in the file's order winning.
func TestTheFirstKeyOfAMatchingHashWins(t *testing.T) {
	base := t.TempDir()
	first, second := "/srv/first", "/srv/second"
	digest := func(text string) string {
		sum := sha256.Sum256([]byte(text))
		return hex.EncodeToString(sum[:])
	}
	write(t, filepath.Join(base, "projects.json"),
		`{"projects": {"/srv/first": "x", "/srv/second": "y"}}`)
	locate := func(names ...string) string { return filepath.Join(append([]string{base}, names...)...) }
	tests := []struct{ name, slug, hash, want string }{
		{"a hash slug", digest(second), "", second},
		{"the session's hash", "other", digest(second), second},
		{"of a slug and a hash, the earlier key", digest(second), digest(first), first},
		{"a slug that is no hash is not looked for", "other", "", ""},
		{"an unknown hash is itself", "other", digest("/srv/none"), digest("/srv/none")},
	}
	subject := New()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var problems parserkit.Problems
			if got := subject.resolveProject(nil, locate, test.slug, test.hash, &problems); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
