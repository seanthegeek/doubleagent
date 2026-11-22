//go:build unix

package summary

import (
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/volumes"
)

// reportCase is a finished run, and the collection.json and the stdout
// summary that describe it.
type reportCase struct {
	name       string
	summary    Summary
	collection string
	result     string
}

// bashrcHints are the two relocation hints read from alice's .bashrc.
func bashrcHints() []Hint {
	source := "/img/root/home/alice/.bashrc"
	return []Hint{
		{User: "alice", Source: source, Line: 1, Variable: "CLAUDE_CONFIG_DIR", Agent: "claude-code"},
		{User: "alice", Source: source, Line: 2, Variable: "CODEX_HOME", Agent: "codex-cli"},
	}
}

// reportCases returns a run over every home and the Docker volumes, and a run
// of one user with every option that changes what is collected.
func reportCases() []reportCase {
	const archive = "testhost_20260101T000000Z_agent-artifacts"
	return []reportCase{
		{"every home and the Docker volumes", Summary{
			Version: "1.22.0", Hostname: "testhost", Mode: "image", Root: "/img/root",
			Platform: "Linux 6.8.0 x86_64", Identity: Identity{UID: 1000},
			Started: "2026-01-01T00:00:00Z", Finished: "2026-01-01T00:00:00Z",
			Options: Options{MaxFileBytes: 1048576, Projects: []string{}, EnvScanMaxSize: 1048576},
			Plutil:  true, Users: []string{"alice", "bob"},
			Homes:       []string{"/img/root/home/alice", "/img/root/home/bob"},
			ActiveUsers: []string{"alice"}, Projects: []string{},
			Counts: manifest.Counts{ByStatus: map[manifest.Status]int{
				manifest.Collected: 4, manifest.SkippedSize: 1, manifest.SkippedUnmatchedVolume: 1,
			}, CollectedBytes: 79},
			Docker:     volumes.Counts{Seen: true, Found: 2, Collected: 1, Desktop: true},
			Relocation: Relocation{Hints: bashrcHints(), RegistryReason: "not Windows", FilesSearched: 2},
			Notes: []string{"docker: Docker Desktop data at " +
				"/img/root/home/alice/Library/Containers/com.docker.docker; " +
				"volumes inside its virtual machine disk are not collected"},
			Archive: archive + ".tar.gz", ArchivePath: "/out/" + archive + ".tar.gz",
			ArchiveSize:  2140,
			ArchiveSHA:   "66dc0374b97dc17499595d87a056311bd4db9523503ce69c6cbadd86633818c7",
			ManifestPath: "/out/" + archive + ".manifest.jsonl",
			SummaryPath:  "/out/" + archive + ".collection.json",
			LogPath:      "/out/" + archive + ".log",
		}, wantCollectionEveryHome, wantResultEveryHome},
		{"one user with an extra project and every option", Summary{
			Version: "1.22.0", Hostname: "testhost", Mode: "image", Root: "/img/root",
			Platform: "Linux 6.8.0 x86_64", Identity: Identity{UID: 1000},
			Started: "2026-01-01T00:00:00Z", Finished: "2026-01-01T00:00:01Z",
			Options: Options{
				Full: true, NoSecrets: true, UsersFilter: "alice", NoDocker: true,
				Projects: []string{"/img/proj"}, EnvScanHome: true, EnvScanMaxSize: 1048576,
			},
			Plutil: true, Users: []string{"alice"}, Homes: []string{"/img/root/home/alice"},
			ActiveUsers: []string{"alice"}, Projects: []string{"/img/proj"},
			Counts: manifest.Counts{ByStatus: map[manifest.Status]int{
				manifest.Collected: 1, manifest.SkippedSecret: 3,
			}, CollectedBytes: 2097152},
			Relocation: Relocation{Hints: bashrcHints(), RegistryReason: "not Windows", FilesSearched: 2},
			Notes:      []string{},
			Archive:    archive + ".tar.gz", ArchivePath: "/out/" + archive + ".tar.gz",
			ArchiveSize:  3787,
			ArchiveSHA:   "b8f654d2c474215358e4efd01d3beb04a59b362e62f8c0f5b24a27d79886bbfc",
			ManifestPath: "/out/" + archive + ".manifest.jsonl",
			SummaryPath:  "/out/" + archive + ".collection.json",
			LogPath:      "/out/" + archive + ".log",
		}, wantCollectionOneUser, wantResultOneUser},
	}
}

func TestCollectionJSONAndResultOfRuns(t *testing.T) {
	t.Parallel()
	for _, test := range reportCases() {
		t.Run(test.name, func(t *testing.T) {
			var collection, result strings.Builder
			if err := WriteCollectionJSON(&collection, test.summary); err != nil {
				t.Fatal(err)
			}
			if collection.String() != test.collection {
				t.Errorf("collection.json:\n%s\nwant:\n%s", collection.String(), test.collection)
			}
			if err := PrintResult(&result, test.summary); err != nil {
				t.Fatal(err)
			}
			if result.String() != test.result {
				t.Errorf("summary:\n%s\nwant:\n%s", result.String(), test.result)
			}
		})
	}
}
