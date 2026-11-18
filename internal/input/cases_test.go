package input

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// bzip2Archive is a bzip2 tar holding alice/.claude/history.jsonl, written
// by another tar writer, since Go's library has no bzip2 writer.
const bzip2Archive = "425a6839314159265359195c55a10000727d80c9100000c001ff8002006e759e2a" +
	"08082000751147947a81900dea80d0c80929a53ca184f5193466841ea7a959d8321047821087ef2e23e4e82043" +
	"97e63a5264a3cdc88b31110422095a3b0ce9954189a8a8cc9d00fd63cbc2f4be2f708b1122c6a90a7cad714907" +
	"f177245385090195c55a10"

// syntheticCatalog is the analyzer's small test catalog for project rows.
const syntheticCatalog = `# home artifacts (agent|glob relative to home)
claude-code|.claude
pi|.pi

# project artifacts (relative to each discovered project)
claude-code|.claude
project|AGENTS.md
pi|.pi
pi-plans|.pi/approved-plan.md
aider|.aider.*
`

// inputCase is one input built under a test directory, opened, and checked;
// TestOpenGivesTheRecordedDumpOfEachCase also compares all it opens to the
// dump recorded under its name in testdata/case_dumps.txt.
type inputCase struct {
	name    string
	catalog string
	host    string
	build   func(t *testing.T, dir string) string
	check   func(t *testing.T, collection *Collection, err error)
}

// opened fails the test unless the input opened.
func opened(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
}

// homeDirOf returns the only home's directory relative to the root, with
// slashes.
func homeDirOf(t *testing.T, collection *Collection) string {
	t.Helper()
	if len(collection.Homes) != 1 {
		t.Fatalf("%d homes, want 1", len(collection.Homes))
	}
	rel, err := filepath.Rel(collection.Root, collection.Homes[0].DiskPath)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(rel)
}

// stagedKeys returns the only home's staged paths, sorted.
func stagedKeys(collection *Collection) []string {
	var keys []string
	for key := range collection.Homes[0].StagedPaths {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// agentsByRel maps each artifact's Rel to its agent.
func agentsByRel(collection *Collection) map[string]string {
	agents := map[string]string{}
	for _, artifact := range collection.Artifacts {
		agents[artifact.Rel] = artifact.Agent
	}
	return agents
}

// wantEqual fails the test when got and want differ.
func wantEqual[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
}

// wantSlice fails the test when got and want differ.
func wantSlice(t *testing.T, what string, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("%s = %q, want %q", what, got, want)
	}
}

// homeDirCase returns a case of one home whose rows are (path,
// archive_path) pairs and whose home directory must come out as want.
func homeDirCase(name, home string, rows [][2]string, summary, want string) inputCase {
	return inputCase{name: name,
		build: func(t *testing.T, dir string) string {
			var lines []string
			for _, row := range rows {
				writeFile(t, filepath.Join(dir, filepath.FromSlash(row[1])), "[]")
				lines = append(lines, collectedRow("bob", home, "pi", row[0], row[1]))
			}
			writeManifest(t, dir, lines...)
			if summary != "" {
				writeFile(t, filepath.Join(dir, "collection.json"), summary)
			}
			return dir
		},
		check: func(t *testing.T, collection *Collection, err error) {
			opened(t, err)
			wantEqual(t, "home directory", homeDirOf(t, collection), want)
		}}
}

// homeDirCases are the home-directory derivations of the analyzer's tests.
func homeDirCases() []inputCase {
	return []inputCase{
		homeDirCase("a Windows row staged at its physical path", `C:\Users\bob`,
			[][2]string{{`C:\Users\bob\.pi\agent\x`, "fs/C/Users/bob/dotfiles/pi/agent/x"}}, "", "fs/C/Users/bob"),
		homeDirCase("a lower-case drive is upper-cased as the collector writes it", `c:\Users\bob`,
			[][2]string{{`c:\Users\bob\.pi\agent\x`, "fs/C/Users/bob/dotfiles/pi/agent/x"}}, "", "fs/C/Users/bob"),
		homeDirCase("a POSIX row staged at its physical path", "/home/bob",
			[][2]string{{"/home/bob/.pi/agent/x", "fs/home/bob/dotfiles/pi/agent/x"}}, "", "fs/home/bob"),
		homeDirCase("an image home whose first row is staged physically", "/mnt/img/home/bob",
			[][2]string{{"/mnt/img/home/bob/.pi/agent/x", "fs/home/bob/dotfiles/pi/agent/x"}},
			`{"mode": "image", "root": "/mnt/img"}`, "fs/home/bob"),
		homeDirCase("a Windows image home mounted on Linux", "/mnt/img/Users/bob",
			[][2]string{{"/mnt/img/Users/bob/.pi/agent/x", "fs/Users/bob/dotfiles/pi/agent/x"}},
			`{"mode": "image", "root": "/mnt/img/"}`, "fs/Users/bob"),
		homeDirCase("a stow-style link is not the target directory", "/home/bob",
			[][2]string{{"/home/bob/.config/gh/hosts.yml", "fs/home/bob/dotfiles/.config/gh/hosts.yml"}},
			`{"mode": "live", "root": "/"}`, "fs/home/bob"),
		homeDirCase("a stow-style link without a summary", "/home/bob",
			[][2]string{{"/home/bob/.config/gh/hosts.yml", "fs/home/bob/dotfiles/.config/gh/hosts.yml"}},
			"", "fs/home/bob"),
		homeDirCase("the first row ending with its relative path names the directory", "/mnt/img/home/bob",
			[][2]string{{"/mnt/img/home/bob/.pi/agent/x", "fs/home/bob/dotfiles/pi/agent/x"},
				{"/mnt/img/home/bob/.claude/history.jsonl", "fs/home/bob/.claude/history.jsonl"}},
			"", "fs/home/bob"),
	}
}

// stagedRow is a (path, archive_path, status) row of a staged-path case;
// a collected row's member is written, a symlink row gets none.
type stagedRow struct{ path, archivePath, status string }

// stagedCase returns a case of one home built from rows whose staged paths
// must be want and whose PathOf must give each rel its archive path.
func stagedCase(name, home string, rows []stagedRow, want []string, pathOf map[string]string) inputCase {
	return inputCase{name: name,
		build: func(t *testing.T, dir string) string {
			var lines []string
			for _, row := range rows {
				kind := "symlink"
				if row.status == "collected" {
					kind = "file"
					writeFile(t, filepath.Join(dir, filepath.FromSlash(row.archivePath)), "[]")
				}
				lines = append(lines, jsonRow("type", kind, "status", row.status, "user", "bob",
					"home", home, "path", row.path, "archive_path", row.archivePath, "agent", "pi"))
			}
			writeManifest(t, dir, lines...)
			return dir
		},
		check: func(t *testing.T, collection *Collection, err error) {
			opened(t, err)
			homeDirOf(t, collection)
			wantSlice(t, "staged paths", stagedKeys(collection), want)
			for rel, archivePath := range pathOf {
				wantEqual(t, "PathOf("+rel+")", collection.Homes[0].PathOf(rel), joinPath(collection.Root, archivePath))
			}
		}}
}

// physicalRows is a home with a link .pi whose files the collector archived
// at the link target's physical path.
func physicalRows() []stagedRow {
	return []stagedRow{
		{"/home/bob/.claude/history.jsonl", "fs/home/bob/.claude/history.jsonl", "collected"},
		{"/home/bob/.pi", "fs/home/bob/.pi", "symlink"},
		{"/home/bob/.pi/agent/sessions/p/s.jsonl", "fs/home/bob/dotfiles/pi/agent/sessions/p/s.jsonl", "collected"},
	}
}

// stagedCases are the staged-path cases of the analyzer's tests.
func stagedCases() []inputCase {
	hosts := stagedRow{"/home/bob/.config/gh/hosts.yml", "fs/home/bob/dotfiles/.config/gh/hosts.yml", "collected"}
	cursor := stagedRow{"/home/bob/.config/Cursor/User/globalStorage/state.vscdb",
		"fs/home/bob/.config/Cursor/User/globalStorage/state.vscdb", "collected"}
	return []inputCase{
		stagedCase("a physically staged file and the directories above it", "/home/bob", physicalRows(),
			[]string{".pi", ".pi/agent", ".pi/agent/sessions", ".pi/agent/sessions/p", ".pi/agent/sessions/p/s.jsonl"},
			map[string]string{
				".pi/agent/sessions/p/s.jsonl":    "fs/home/bob/dotfiles/pi/agent/sessions/p/s.jsonl",
				".pi":                             "fs/home/bob/dotfiles/pi",
				".pi/agent/sessions/other/t.json": "fs/home/bob/dotfiles/pi/agent/sessions/other/t.json",
				".claude/history.jsonl":           "fs/home/bob/.claude/history.jsonl",
				"dotfiles":                        "fs/home/bob/dotfiles",
				"":                                "fs/home/bob",
			}),
		stagedCase("a link named like its target, with its row", "/home/bob",
			[]stagedRow{{"/home/bob/.config", "fs/home/bob/.config", "symlink"}, hosts},
			[]string{".config", ".config/gh", ".config/gh/hosts.yml"},
			map[string]string{".config/gh": "fs/home/bob/dotfiles/.config/gh", ".bashrc": "fs/home/bob/.bashrc"}),
		stagedCase("a link named like its target, without its row", "/home/bob", []stagedRow{hosts},
			[]string{".config", ".config/gh", ".config/gh/hosts.yml"},
			map[string]string{".config/gh": "fs/home/bob/dotfiles/.config/gh"}),
		stagedCase("a link row without an archive member still bounds the map", "/home/bob",
			[]stagedRow{{"/home/bob/.config/gh", "", "symlink"}, hosts, cursor},
			[]string{".config/gh", ".config/gh/hosts.yml"},
			map[string]string{".config/Cursor/User/globalStorage/state.vscdb": cursor.archivePath}),
		stagedCase("a nested link maps the directory holding its files", "/home/bob",
			[]stagedRow{{"/home/bob/.config", "fs/home/bob/.config", "symlink"},
				{"/home/bob/.config/gh", "fs/home/bob/dotfiles/.config/gh", "symlink"},
				{"/home/bob/.config/gh/hosts.yml", "fs/home/bob/credentials/gh/hosts.yml", "collected"}},
			[]string{".config", ".config/gh", ".config/gh/hosts.yml"},
			map[string]string{".config/gh": "fs/home/bob/credentials/gh", ".config": "fs/home/bob/dotfiles/.config"}),
		stagedCase("a link below a real directory maps from the link down", "/home/bob",
			[]stagedRow{{"/home/bob/.config/gh", "fs/home/bob/.config/gh", "symlink"}, hosts, cursor},
			[]string{".config/gh", ".config/gh/hosts.yml"},
			map[string]string{".config/gh": "fs/home/bob/dotfiles/.config/gh"}),
		stagedCase("a link below an excluded link leaves the real directory in place", "/home/dot",
			[]stagedRow{{"/home/dot/.copilot/logs", "fs/home/dot/.copilot/logs", "symlink"},
				{"/home/dot/.copilot/repos/r1", "fs/home/dot/dotfiles/repos/r1", "symlink"},
				{"/home/dot/.copilot/session-state/s1/workspace.yaml",
					"fs/home/dot/.copilot/session-state/s1/workspace.yaml", "collected"}},
			[]string{".copilot/repos", ".copilot/repos/r1"},
			map[string]string{".copilot/session-state/s1/workspace.yaml": "fs/home/dot/.copilot/session-state/s1/workspace.yaml"}),
		stagedCase("a Windows home staged physically", `C:\Users\bob`,
			[]stagedRow{{`C:\Users\bob\.claude\x`, "fs/C/Users/bob/.claude/x", "collected"},
				{`C:\Users\bob\.pi\agent\x`, "fs/C/Users/bob/dotfiles/pi/agent/x", "collected"}},
			[]string{".pi", ".pi/agent", ".pi/agent/x"},
			map[string]string{".pi/agent": "fs/C/Users/bob/dotfiles/pi/agent", ".claude/x": "fs/C/Users/bob/.claude/x"}),
		stagedCase("a home with no physical rows maps nothing", "/home/bob",
			[]stagedRow{{"/home/bob/.claude/history.jsonl", "fs/home/bob/.claude/history.jsonl", "collected"},
				{"/home/bob/.pi", "fs/home/bob/.pi", "symlink"}},
			nil, map[string]string{".pi/agent": "fs/home/bob/.pi/agent"}),
	}
}

// manifestCases cover the manifest and collection.json rules.
func manifestCases() []inputCase {
	record := jsonRow("path", "/x", "status", "collected", "type", "file")
	var cases []inputCase
	for _, space := range []string{"\u00a0", "\u2028", "\u0085"} {
		cases = append(cases, inputCase{name: "a manifest line is trimmed of JSON whitespace only " + strings.TrimSpace(space),
			build: func(t *testing.T, dir string) string {
				writeManifest(t, dir, " "+record+"\t\r", space+record)
				return dir
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantSlice(t, "notes", collection.Notes, []string{"1 manifest rows did not parse"})
				wantSlice(t, "problems", collection.Problems, []string{"/x: missing from the collection"})
			}})
	}
	cases = append(cases, projectAgentsCase(true), projectAgentsCase(false))
	return append(cases, collectionCases()...)
}

// collectionCases cover collection.json, project rows and missing files.
func collectionCases() []inputCase {
	return slices.Concat(collectionSummaryCases(), collectionDamageCases())
}

// collectionSummaryCases cover collection.json, project rows and the host option.
func collectionSummaryCases() []inputCase {
	return []inputCase{
		{name: "collection.json gives the host and marks its projects", catalog: syntheticCatalog,
			build: func(t *testing.T, dir string) string {
				writeTree(t, dir, map[string]string{"fs/home/alice/.claude/history.jsonl": "",
					"fs/srv/proj/.claude/settings.local.json": "{}", "fs/srv/proj/AGENTS.md": "# x\n"})
				writeManifest(t, dir,
					collectedRow("alice", "/home/alice", "claude-code", "/home/alice/.claude/history.jsonl", "fs/home/alice/.claude/history.jsonl"),
					collectedRow("alice", "/srv/proj", "claude-code", "/srv/proj/.claude/settings.local.json", "fs/srv/proj/.claude/settings.local.json"),
					collectedRow("alice", "/srv/proj", "project", "/srv/proj/AGENTS.md", "fs/srv/proj/AGENTS.md"))
				writeFile(t, filepath.Join(dir, "collection.json"), `{"hostname": "h1", "projects": ["/srv/proj"]}`)
				return dir
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantEqual(t, "kind", collection.Kind, KindCollected)
				wantEqual(t, "host", collection.Host, "h1")
				wantEqual(t, "homes", len(collection.Homes), 2)
				wantEqual(t, "home project", collection.Homes[0].Project, false)
				wantEqual(t, "project project", collection.Homes[1].Project, true)
				wantEqual(t, "AGENTS.md agent", agentsByRel(collection)["AGENTS.md"], "project")
				wantEqual(t, "size", collection.Artifacts[0].Size(), int64(2))
			}},
		{name: "a collection before 1.12.0 is re-attributed through the project catalog", catalog: syntheticCatalog,
			build: func(t *testing.T, dir string) string {
				rows := []string{collectedRow("alice", "/home/alice", "pi", "/home/alice/.pi/agent/settings.json", "fs/home/alice/.pi/agent/settings.json")}
				writeFile(t, filepath.Join(dir, "fs/home/alice/.pi/agent/settings.json"), "{}")
				for _, rel := range []string{".claude/settings.local.json", "AGENTS.md", ".pi/approved-plan.md",
					".pi/notes.txt", ".aider.input.history", "unknown.txt"} {
					writeFile(t, filepath.Join(dir, "fs/srv/proj", rel), "{}")
					rows = append(rows, collectedRow("alice", "/srv/proj", "project", "/srv/proj/"+rel, "fs/srv/proj/"+rel))
				}
				writeManifest(t, dir, rows...)
				writeFile(t, filepath.Join(dir, "collection.json"), `{"hostname": "h1"}`)
				return dir
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantEqual(t, "home project", collection.Homes[0].Project, false)
				wantEqual(t, "project project", collection.Homes[1].Project, true)
				agents := agentsByRel(collection)
				for rel, agent := range map[string]string{".claude/settings.local.json": "claude-code",
					"AGENTS.md": "project", ".pi/approved-plan.md": "pi-plans", ".pi/notes.txt": "pi",
					".aider.input.history": "aider", "unknown.txt": "project"} {
					wantEqual(t, rel, agents[rel], agent)
				}
			}},
		{name: "the host option overrides collection.json", host: "override",
			build: func(t *testing.T, dir string) string {
				writeFile(t, filepath.Join(dir, "fs/home/a/.claude/x"), "x")
				writeManifest(t, dir, collectedRow("a", "/home/a", "claude-code", "/home/a/.claude/x", "fs/home/a/.claude/x"))
				writeFile(t, filepath.Join(dir, "collection.json"), `{"hostname": "h1"}`)
				return dir
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantEqual(t, "host", collection.Host, "override")
				wantEqual(t, "home host", collection.Homes[0].Host, "override")
			}},
	}
}

// collectionDamageCases cover missing files and unreadable summaries.
func collectionDamageCases() []inputCase {
	return []inputCase{
		{name: "a collected file absent from fs is a problem",
			build: func(t *testing.T, dir string) string {
				writeManifest(t, dir, collectedRow("a", "/home/a", "claude-code", "/home/a/.claude/x", "fs/home/a/.claude/x"),
					jsonRow("path", "/home/a/.claude/y", "user", "a", "home", "/home/a", "status", "collected", "type", "file"))
				return dir
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantSlice(t, "problems", collection.Problems, []string{"/home/a/.claude/x: missing from the collection",
					"/home/a/.claude/y: missing from the collection"})
				wantEqual(t, "artifacts", len(collection.Artifacts), 0)
				wantEqual(t, "homes", len(collection.Homes), 1)
			}},
		{name: "unparseable collection.json leaves a note",
			build: func(t *testing.T, dir string) string {
				writeManifest(t, dir)
				writeFile(t, filepath.Join(dir, "collection.json"), `{"hostname": `)
				return dir
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				if len(collection.Notes) != 1 || !strings.HasPrefix(collection.Notes[0], "collection.json unreadable: ") {
					t.Fatalf("notes %q", collection.Notes)
				}
			}},
		{name: "a false collection.json value is no summary and no note",
			build: func(t *testing.T, dir string) string {
				writeManifest(t, dir)
				writeFile(t, filepath.Join(dir, "collection.json"), `[]`)
				return dir
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantEqual(t, "notes", len(collection.Notes), 0)
				wantEqual(t, "summary is null", collection.Summary.IsNull(), true)
			}},
	}
}

// projectAgentsCase is the collection of the analyzer's project-agents
// tests, in the shape of collector 1.12.0 (newShape) or of an earlier one:
// no projects key and every project row of agent project.
func projectAgentsCase(newShape bool) inputCase {
	name := "a collection of collector 1.12.0 with two projects"
	if !newShape {
		name = "the same collection from an earlier collector"
	}
	return inputCase{name: name, catalog: syntheticCatalog,
		build: func(t *testing.T, dir string) string {
			summary := `{"hostname": "h1"}`
			if newShape {
				summary = `{"hostname": "h1", "projects": ["/srv/proj", "/srv/shared"]}`
			}
			writeFile(t, filepath.Join(dir, "collection.json"), summary)
			files := [][4]string{{"/home/alice", "claude-code", ".claude/settings.json", "{}"},
				{"/srv/proj", "claude-code", ".claude/settings.local.json", "{}"},
				{"/srv/proj", "aider", ".aider.input.history", "# 2026-10-01\n+hello\n"},
				{"/srv/proj", "project", "AGENTS.md", "# x\n"}, {"/srv/shared", "project", "AGENTS.md", "# shared\n"}}
			var rows []string
			for _, file := range files {
				agent := file[1]
				if !newShape && file[0] != "/home/alice" {
					agent = ProjectAgent
				}
				path := file[0] + "/" + file[2]
				writeFile(t, filepath.Join(dir, "fs"+path), file[3])
				rows = append(rows, collectedRow("alice", file[0], agent, path, "fs"+path))
			}
			writeManifest(t, dir, rows...)
			return dir
		},
		check: func(t *testing.T, collection *Collection, err error) {
			opened(t, err)
			marks := map[string]bool{}
			for _, home := range collection.Homes {
				marks[home.Original] = home.Project
			}
			wantEqual(t, "home marked", marks["/home/alice"], false)
			wantEqual(t, "project marked", marks["/srv/proj"], true)
			wantEqual(t, "shared project marked", marks["/srv/shared"], true)
			wantEqual(t, "aider", agentsByRel(collection)[".aider.input.history"], "aider")
		}}
}

// damagedManifestCases are manifests with rows that are not objects, sizes
// that are not numbers, a collection.json that is not an object and archive
// paths leaving the collection; the binary counts or refuses them.
func damagedManifestCases() []inputCase {
	return []inputCase{
		{name: "a line that is JSON but not an object, and an unreadable size, are bad rows",
			build: func(t *testing.T, dir string) string {
				writeFile(t, filepath.Join(dir, "fs/home/a/.claude/x"), "x")
				good := jsonRow("path", "/home/a/.claude/x", "home", "/home/a", "archive_path", "fs/home/a/.claude/x",
					"status", "collected", "type", "file", "size", " 12 ")
				badSize := jsonRow("path", "/home/a/.claude/y", "status", "collected", "type", "file", "size", "1.5")
				skippedSize := jsonRow("path", "/home/a/.claude/z", "status", "skipped_size", "size", "big")
				writeManifest(t, dir, "[1]", `"row"`, good, badSize, skippedSize)
				return dir
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantSlice(t, "notes", collection.Notes, []string{"3 manifest rows did not parse"})
				wantEqual(t, "rows kept", len(collection.Manifest), 2)
				wantEqual(t, "size", collection.Artifacts[0].Size(), int64(12))
			}},
		{name: "a true collection.json that is not an object leaves a note",
			build: func(t *testing.T, dir string) string {
				writeManifest(t, dir)
				writeFile(t, filepath.Join(dir, "collection.json"), `[1]`)
				return dir
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantSlice(t, "notes", collection.Notes, []string{"collection.json unreadable: not a JSON object"})
			}},
		{name: "an archive path leaving the collection is a problem",
			build: func(t *testing.T, dir string) string {
				collection := filepath.Join(dir, "collection")
				writeFile(t, filepath.Join(dir, "outside"), "secret")
				writeManifest(t, collection,
					collectedRow("a", "/home/a", "claude-code", "/home/a/.claude/x", "../outside"),
					collectedRow("a", "/home/a", "claude-code", "/home/a/.claude/y", filepath.ToSlash(filepath.Join(dir, "outside"))))
				return collection
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantSlice(t, "problems", collection.Problems, []string{"/home/a/.claude/x: outside the collection",
					"/home/a/.claude/y: outside the collection"})
			}},
	}
}

// looseHome returns the files of a small home tree, by path below the test
// directory.
func looseHome() map[string]string {
	return map[string]string{
		"home/alice/.claude/history.jsonl":                 "{}\n",
		"home/alice/.claude/settings.json":                 "{}",
		"home/alice/.claude-work/history.jsonl":            "{}\n",
		"home/alice/.gemini/settings.json":                 "{}",
		"home/alice/.gemini/antigravity-cli/history.jsonl": "{}\n",
		"home/alice/.codex/history.jsonl":                  "{}\n",
		"home/alice/Documents/notes.txt":                   "not an agent file\n",
		"home/alice/dev/proj/.claude/settings.json":        "{}",
		"home/alice/dev/proj/AGENTS.md":                    "# x\n",
	}
}

// looseCases cover discovery in trees without a manifest.
func looseCases() []inputCase {
	return slices.Concat(looseHomeCases(), looseShapeCases())
}

// looseHomeCases cover homes found in a tree and agent directories as roots.
func looseHomeCases() []inputCase {
	return []inputCase{
		{name: "a home tree attributes each file to the most specific entry",
			build: func(t *testing.T, dir string) string {
				writeTree(t, dir, looseHome())
				return filepath.Join(dir, "home")
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantEqual(t, "kind", collection.Kind, KindLoose)
				wantEqual(t, "homes", len(collection.Homes), 1)
				wantEqual(t, "user", collection.Homes[0].User, "alice")
				wantEqual(t, "original", collection.Homes[0].Original, "/alice")
				agents := agentsByRel(collection)
				wantEqual(t, "nested", agents[".gemini/antigravity-cli/history.jsonl"], "antigravity")
				wantEqual(t, "enclosing", agents[".gemini/settings.json"], "gemini-cli")
				wantEqual(t, "config home", agents[".claude-work/history.jsonl"], "claude-code")
				if _, found := agents["dev/proj/.claude/settings.json"]; found {
					t.Fatal("a project inside the home was attributed")
				}
				if _, found := agents["Documents/notes.txt"]; found {
					t.Fatal("an unrelated file was attributed")
				}
			}},
		{name: "an agent directory as the root makes its parent the home",
			build: func(t *testing.T, dir string) string {
				writeTree(t, dir, looseHome())
				return filepath.Join(dir, "home", "alice", ".claude")
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantEqual(t, "user", collection.Homes[0].User, "alice")
				wantSlice(t, "notes", collection.Notes, []string{"root is a claude-code directory; treating its parent as the home"})
				for _, artifact := range collection.Artifacts {
					if !strings.HasPrefix(artifact.Rel, ".claude/") || artifact.Agent != "claude-code" {
						t.Fatalf("artifact %q of %s", artifact.Rel, artifact.Agent)
					}
				}
			}},
		{name: "a gemini directory as the root keeps its nested antigravity entry",
			build: func(t *testing.T, dir string) string {
				writeTree(t, dir, looseHome())
				return filepath.Join(dir, "home", "alice", ".gemini")
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				agents := agentsByRel(collection)
				wantEqual(t, "nested", agents[".gemini/antigravity-cli/history.jsonl"], "antigravity")
				wantEqual(t, "artifacts", len(collection.Artifacts), 2)
			}},
	}
}

// looseShapeCases cover trees that are not homes, bare homes, images and fs/ alone.
func looseShapeCases() []inputCase {
	return []inputCase{
		{name: "a directory holding only shared files is not a home",
			build: func(t *testing.T, dir string) string {
				writeTree(t, dir, map[string]string{"repo/AGENTS.md": "x", "repo/.env": "A=1"})
				return filepath.Join(dir, "repo")
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantEqual(t, "homes", len(collection.Homes), 0)
			}},
		{name: "a bare home names no user",
			build: func(t *testing.T, dir string) string {
				writeTree(t, dir, looseHome())
				return filepath.Join(dir, "home", "alice")
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantEqual(t, "user", collection.Homes[0].User, "")
				wantEqual(t, "original", collection.Homes[0].Original, "")
			}},
		{name: "an image finds the users of each home convention",
			build: func(t *testing.T, dir string) string {
				writeTree(t, dir, map[string]string{"image/home/alice/.claude/x": "x",
					"image/Users/carol/.codex/y": "y", "image/root/.claude/z": "z", "image/var/www/.claude/w": "w"})
				return filepath.Join(dir, "image") + string(filepath.Separator)
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				users := map[string]string{}
				for _, home := range collection.Homes {
					users[home.Original] = home.User
					wantEqual(t, "inferred", home.Inferred, true)
				}
				wantEqual(t, "alice", users["/home/alice"], "alice")
				wantEqual(t, "carol", users["/Users/carol"], "carol")
				wantEqual(t, "root", users["/root"], "root")
				wantEqual(t, "www", users["/var/www"], "www")
				wantEqual(t, "source is as given", strings.HasSuffix(collection.Source, "image"+string(filepath.Separator)), true)
			}},
		{name: "fs without a manifest is taken as the root",
			build: func(t *testing.T, dir string) string {
				writeTree(t, dir, map[string]string{"extracted/fs/home/alice/.claude/x": "x"})
				return filepath.Join(dir, "extracted")
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantSlice(t, "notes", collection.Notes, []string{"fs/ found without manifest.jsonl; treating fs/ as the root"})
				wantEqual(t, "original", collection.Homes[0].Original, "/home/alice")
			}},
	}
}

// archiveCases cover tar and zip inputs.
func archiveCases() []inputCase {
	return slices.Concat(archiveCollectionCases(), archiveShapeCases(), archiveMemberCases(), zipMemberCases())
}

// archiveCollectionCases cover archives of collections and of homes.
func archiveCollectionCases() []inputCase {
	collection := func(t *testing.T, root string) {
		writeFile(t, filepath.Join(root, "fs/home/alice/.claude/history.jsonl"), "{}\n")
		writeManifest(t, root, collectedRow("alice", "/home/alice", "claude-code",
			"/home/alice/.claude/history.jsonl", "fs/home/alice/.claude/history.jsonl"))
		writeFile(t, filepath.Join(root, "collection.json"), `{"hostname": "h1"}`)
	}
	return []inputCase{
		{name: "a gzip tar of a collection is read from its manifest",
			build: func(t *testing.T, dir string) string {
				collection(t, filepath.Join(dir, "staging"))
				archive := filepath.Join(dir, "h1_agent-artifacts.tar.gz")
				writeFile(t, archive, string(tarBytes(t, true, treeMembers(t, filepath.Join(dir, "staging"))...)))
				return archive
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantEqual(t, "kind", collection.Kind, KindArchive)
				wantEqual(t, "host", collection.Host, "h1")
				wantEqual(t, "artifacts", len(collection.Artifacts), 1)
				if !strings.HasPrefix(filepath.Base(collection.ExtractedDir()), "cac-analyzer-") {
					t.Fatalf("extraction directory %s", collection.ExtractedDir())
				}
			}},
		{name: "a zip of a collection is read from its manifest",
			build: func(t *testing.T, dir string) string {
				collection(t, filepath.Join(dir, "staging"))
				var names []string
				bodies := map[string]string{}
				for _, member := range treeMembers(t, filepath.Join(dir, "staging")) {
					names = append(names, member.name)
					bodies[member.name] = member.body
				}
				archive := filepath.Join(dir, "h1_agent-artifacts.zip")
				writeFile(t, archive, string(zipBytes(t, names, bodies)))
				return archive
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantEqual(t, "kind", collection.Kind, KindArchive)
				wantEqual(t, "artifacts", len(collection.Artifacts), 1)
			}},
		{name: "a tar of a home without a manifest is loose",
			build: func(t *testing.T, dir string) string {
				writeTree(t, dir, map[string]string{"tree/alice/.claude/history.jsonl": "{}\n", "tree/alice/notes": "x"})
				archive := filepath.Join(dir, "home.tar")
				writeFile(t, archive, string(tarBytes(t, false, treeMembers(t, filepath.Join(dir, "tree"))...)))
				return archive
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantEqual(t, "kind", collection.Kind, KindLoose)
				wantEqual(t, "user", collection.Homes[0].User, "alice")
				info, err := os.Lstat(collection.Artifacts[0].DiskPath)
				opened(t, err)
				wantEqual(t, "member time", info.ModTime().Unix(), int64(1700000000))
			}},
	}
}

// archiveShapeCases cover bzip2, empty and refused archives.
func archiveShapeCases() []inputCase {
	return []inputCase{
		{name: "a bzip2 tar is read",
			build: func(t *testing.T, dir string) string {
				archive := filepath.Join(dir, "home.tar.bz2")
				data, err := hex.DecodeString(bzip2Archive)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, archive, string(data))
				return archive
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantEqual(t, "artifact", agentsByRel(collection)[".claude/history.jsonl"], "claude-code")
			}},
		{name: "a tar with only its end blocks is an empty loose tree",
			build: func(t *testing.T, dir string) string {
				archive := filepath.Join(dir, "empty.tar")
				writeFile(t, archive, string(make([]byte, 1024)))
				return archive
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantEqual(t, "kind", collection.Kind, KindLoose)
				wantEqual(t, "homes", len(collection.Homes), 0)
			}},
		{name: "a file that is no archive is refused",
			build: func(t *testing.T, dir string) string {
				path := filepath.Join(dir, "notes.txt")
				writeFile(t, path, "not an archive")
				return path
			},
			check: func(t *testing.T, collection *Collection, err error) {
				var inputError *Error
				if !errors.As(err, &inputError) || !strings.HasSuffix(err.Error(), " is neither a tar nor a zip archive") {
					t.Fatalf("error %v", err)
				}
			}},
		{name: "a tar member climbing out of the directory refuses the archive",
			build: func(t *testing.T, dir string) string {
				archive := filepath.Join(dir, "escape.tar.gz")
				writeFile(t, archive, string(tarBytes(t, true, tarMember{name: "a/../../escape.txt", typeflag: '0', body: "x"})))
				return archive
			},
			check: func(t *testing.T, collection *Collection, err error) {
				var inputError *Error
				if !errors.As(err, &inputError) || !strings.Contains(err.Error(), ": cannot extract: ") {
					t.Fatalf("error %v", err)
				}
			}},
	}
}

// archiveMemberCases cover the names and kinds of tar members.
func archiveMemberCases() []inputCase {
	return []inputCase{
		{name: "a tar member's leading slash is dropped and its inner dot-dot resolved",
			build: func(t *testing.T, dir string) string {
				archive := filepath.Join(dir, "abs.tar.gz")
				writeFile(t, archive, string(tarBytes(t, true,
					tarMember{name: "/alice/x/../.claude/history.jsonl", typeflag: '0', body: "{}\n"})))
				return archive
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				wantEqual(t, "rel", collection.Artifacts[0].Rel, ".claude/history.jsonl")
				wantEqual(t, "user", collection.Homes[0].User, "alice")
			}},
		linkMemberCase(),
		{name: "a later member replaces an earlier one of the same name",
			build: func(t *testing.T, dir string) string {
				archive := filepath.Join(dir, "twice.tar.gz")
				writeFile(t, archive, string(tarBytes(t, true,
					tarMember{name: "alice/.claude/x", typeflag: '0', body: "first"},
					tarMember{name: "alice/.claude/x", typeflag: '0', body: "second"})))
				return archive
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				data, err := os.ReadFile(collection.Artifacts[0].DiskPath)
				opened(t, err)
				wantEqual(t, "contents", string(data), "second")
			}},
	}
}

// zipMemberCases cover the names of zip members.
func zipMemberCases() []inputCase {
	return []inputCase{
		{name: "unsafe zip members are skipped",
			build: func(t *testing.T, dir string) string {
				names := []string{"../escape.txt", "/abs.txt", "C:/drive.txt", `alice\.claude\x`, "alice/.claude/./y"}
				archive := filepath.Join(dir, "c.zip")
				writeFile(t, archive, string(zipBytes(t, names, map[string]string{})))
				return archive
			},
			check: func(t *testing.T, collection *Collection, err error) {
				opened(t, err)
				var rels []string
				for _, artifact := range collection.Artifacts {
					rels = append(rels, artifact.Rel)
				}
				slices.Sort(rels)
				wantSlice(t, "artifacts", rels, []string{".claude/x", ".claude/y"})
			}},
		{name: "a zip member below a file member is an error",
			build: func(t *testing.T, dir string) string {
				archive := filepath.Join(dir, "c.zip")
				writeFile(t, archive, string(zipBytes(t, []string{"a", "a/b"}, map[string]string{})))
				return archive
			},
			check: func(t *testing.T, collection *Collection, err error) {
				if err == nil {
					t.Fatal("no error")
				}
			}},
	}
}

// allInputCases returns every case.
func allInputCases() []inputCase {
	var cases []inputCase
	for _, group := range [][]inputCase{homeDirCases(), stagedCases(), manifestCases(),
		damagedManifestCases(), looseCases(), archiveCases(), platformCases()} {
		cases = append(cases, group...)
	}
	return cases
}

// openCase builds and opens one case under a new test directory.
func openCase(t *testing.T, test inputCase) (*Collection, error) {
	t.Helper()
	input := test.build(t, t.TempDir())
	collection, err := Open(input, Options{Matcher: matcherOf(t, test.catalog), Host: test.host})
	if err == nil {
		t.Cleanup(func() { _ = collection.Cleanup() })
	}
	return collection, err
}

func TestOpenReadsEachInput(t *testing.T) {
	for _, test := range allInputCases() {
		t.Run(test.name, func(t *testing.T) {
			collection, err := openCase(t, test)
			test.check(t, collection, err)
		})
	}
}

func TestCleanupRemovesTheExtractionDirectory(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "h.tar")
	writeFile(t, archive, string(tarBytes(t, false, tarMember{name: "alice/.claude/x", typeflag: '0', body: "x"})))
	work := filepath.Join(dir, "work", "deeper")
	collection, err := Open(archive, Options{Matcher: matcherOf(t, ""), WorkDir: work})
	opened(t, err)
	extracted := collection.ExtractedDir()
	if filepath.Dir(extracted) != work || !strings.HasPrefix(filepath.Base(extracted), "cac-") {
		t.Fatalf("extracted to %s, want a cac- directory in %s", extracted, work)
	}
	if err := collection.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(extracted); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("extraction directory still there: %v", err)
	}
	wantEqual(t, "extracted after cleanup", collection.ExtractedDir(), "")
}

func TestOpenRefusesToRunWithoutAMatcher(t *testing.T) {
	if _, err := Open(t.TempDir(), Options{}); err == nil {
		t.Fatal("no error without a matcher")
	}
}

func TestOpenRefusesAMissingInput(t *testing.T) {
	_, err := Open(filepath.Join(t.TempDir(), "nope"), Options{Matcher: matcherOf(t, "")})
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error %v, want not exist", err)
	}
}

func TestSidecarNotes(t *testing.T) {
	cases := []struct {
		name, sidecar, wantPrefix string
	}{
		{"an upper-case digest with a CRLF verifies", "%s  c.tar\r\n", "archive sha256 verified against c.tar.sha256"},
		{"a no-break space does not end the digest", "%s\u00a0c.tar\n", "WARNING archive sha256 "},
		{"another digest warns", "00  c.tar\n", "WARNING archive sha256 "},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			archive := filepath.Join(dir, "c.tar")
			data := tarBytes(t, false, tarMember{name: "alice/.claude/x", typeflag: '0', body: "x"})
			writeFile(t, archive, string(data))
			writeFile(t, archive+".sha256", strings.Replace(test.sidecar, "%s", strings.ToUpper(sha256Hex(data)), 1))
			collection, err := Open(archive, Options{Matcher: matcherOf(t, "")})
			opened(t, err)
			defer collection.Cleanup()
			if len(collection.Notes) != 1 || !strings.HasPrefix(collection.Notes[0], test.wantPrefix) {
				t.Fatalf("notes %q, want one starting %q", collection.Notes, test.wantPrefix)
			}
		})
	}
}

// linkMemberCase is an archive whose link member, were it created, would
// carry the next member's file outside the extraction directory.
func linkMemberCase() inputCase {
	var outside string
	return inputCase{name: "a link member is skipped and nothing is written through it",
		build: func(t *testing.T, dir string) string {
			outside = filepath.Join(dir, "outside")
			if err := os.Mkdir(outside, 0o777); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(dir, "link.tar.gz")
			writeFile(t, archive, string(tarBytes(t, true,
				tarMember{name: "./alice/.claude", typeflag: '2', linkname: outside},
				tarMember{name: "./alice/.claude/k", typeflag: '1', linkname: "../../x"},
				tarMember{name: "./alice/.claude/authorized_keys", typeflag: '0', body: "planted"})))
			return archive
		},
		check: func(t *testing.T, collection *Collection, err error) {
			opened(t, err)
			info, err := os.Lstat(filepath.Dir(collection.Artifacts[0].DiskPath))
			opened(t, err)
			wantEqual(t, "a real directory", info.IsDir(), true)
			wantEqual(t, "artifacts", len(collection.Artifacts), 1)
			entries, err := os.ReadDir(outside)
			opened(t, err)
			wantEqual(t, "files outside", len(entries), 0)
		}}
}

// sha256Hex returns the hex SHA-256 digest of data.
func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
