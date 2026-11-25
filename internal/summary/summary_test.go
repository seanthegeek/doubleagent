package summary

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/users"
	"github.com/seanthegeek/doubleagent/internal/volumes"
)

func TestHumanSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"}, {512, "512 B"}, {1023, "1023 B"}, {1024, "1.0 KiB"},
		{1536, "1.5 KiB"}, {1048576, "1.0 MiB"}, {21913600, "20.9 MiB"},
		{1073741824, "1.0 GiB"}, {1 << 40, "1.0 TiB"}, {1 << 50, "1024.0 TiB"},
		{1048575, "1024.0 KiB"},
	}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			if got := humanSize(test.bytes); got != test.want {
				t.Errorf("humanSize(%d) = %q, want %q", test.bytes, got, test.want)
			}
		})
	}
}

func TestCountNoun(t *testing.T) {
	tests := []struct {
		count int64
		want  string
	}{{0, "0 files"}, {1, "1 file"}, {2, "2 files"}, {1000, "1000 files"}}
	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			if got := countNoun(test.count, "file"); got != test.want {
				t.Errorf("countNoun = %q, want %q", got, test.want)
			}
		})
	}
}

func TestActiveUsers(t *testing.T) {
	homes := []users.Home{{User: "a", Path: "/h/a"}, {User: "b", Path: "/H/B"}, {User: "c", Path: ""}}
	withRows := map[string]bool{"/H/B": true, "": true}
	got := ActiveUsers(homes, func(home string) bool { return withRows[home] })
	if len(got) != 1 || got[0].User != "b" {
		t.Errorf("ActiveUsers = %+v", got)
	}
}

// sampleSummary is a small run with every kind of value the template holds.
func sampleSummary() Summary {
	return Summary{
		Version: "1.2.3", Hostname: `ho"st`, Mode: "live", Platform: "Linux 6 x86_64", Identity: Identity{UID: 1000},
		Started: "2026-01-02T03:04:05Z", Finished: "2026-01-02T03:04:06Z",
		Options: Options{MaxFileBytes: 5, UsersFilter: "a,b", Projects: []string{"/p"}, NoPreserveAtime: true},
		Users:   []string{"a", ""}, Homes: []string{"/h/a", "/h/b"},
		Counts: manifest.Counts{ByStatus: map[manifest.Status]int{manifest.Collected: 2}, CollectedBytes: 9},
		Docker: volumes.Counts{Found: 1},
		Relocation: Relocation{
			Hints:          []Hint{{User: "a", Source: "/x\ty", Line: 3, Variable: "V", Agent: "g", Value: ""}},
			RegistryReason: "not Windows", FilesSearched: 4,
		},
		Notes: []string{"n1"}, Archive: "a.tar.gz",
	}
}

const sampleJSON = `{
  "tool": "doubleagent",
  "version": "1.2.3",
  "hostname": "ho\"st",
  "mode": "live",
  "root": "/",
  "platform": "Linux 6 x86_64",
  "run_as_uid": "1000",
  "started": "2026-01-02T03:04:05Z",
  "finished": "2026-01-02T03:04:06Z",
  "options": {
    "full": false,
    "no_secrets": false,
    "max_file_size_bytes": 5,
    "users_filter": "a,b",
    "no_docker": false,
    "no_projects": false,
    "projects": [
      "/p"
    ],
    "env_scan_home": false,
    "env_scan_max_size_bytes": 0,
    "no_preserve_atime": true
  },
  "capabilities": {
    "plutil": false
  },
  "users": [
    "a",
    ""
  ],
  "homes": [
    "/h/a",
    "/h/b"
  ],
  "users_with_artifacts": [],
  "projects": [],
  "counts": {
    "collected": 2,
    "symlink": 0,
    "skipped_excluded": 0,
    "skipped_size": 0,
    "skipped_secret": 0,
    "skipped_special": 0,
    "error_copy": 0,
    "skipped_unmatched_volume": 0,
    "error_read": 0,
    "collected_bytes": 9
  },
  "docker": {
    "volumes_found": 1,
    "volumes_collected": 0,
    "unreadable": 0,
    "docker_desktop": false
  },
  "relocation": {
    "hints": [
      {
        "user": "a",
        "source": "/x\ty",
        "line": 3,
        "variable": "V",
        "agent": "g",
        "value": ""
      }
    ],
    "registry": {
      "read": false,
      "reason": "not Windows",
      "users_read": [],
      "users_not_loaded": [],
      "users_denied": []
    },
    "files_searched": 4,
    "files_skipped_size": 0
  },
  "notes": [
    "n1"
  ],
  "archive": "a.tar.gz"
}
`

// unixPlatformLines are the lines of sampleJSON that differ by platform:
// the live root and the account.
const unixPlatformLines = "  \"root\": \"/\",\n" + "  \"platform\": \"Linux 6 x86_64\",\n" +
	"  \"run_as_uid\": \"1000\",\n"

func TestWriteCollectionJSONLayout(t *testing.T) {
	var out bytes.Buffer
	if err := WriteCollectionJSON(&out, sampleSummary()); err != nil {
		t.Fatal(err)
	}
	platformLines := "  \"root\": " + quoted(LiveRootText) + ",\n" +
		"  \"platform\": \"Linux 6 x86_64\",\n" + accountLines(sampleSummary().Identity)
	want := strings.Replace(sampleJSON, unixPlatformLines, platformLines, 1)
	if !strings.Contains(sampleJSON, unixPlatformLines) || out.String() != want {
		t.Errorf("collection.json differs:\n%s\nwant:\n%s", out.String(), want)
	}
}

// quoted returns text as a JSON string, written as collection.json writes one.
func quoted(text string) string {
	encoded, err := json.Marshal(text)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func TestWriteCollectionJSONEscapesLikeAnyJSONWriter(t *testing.T) {
	summary := sampleSummary()
	summary.Hostname = "a<b>&c\u2028d\xffe"
	var out bytes.Buffer
	if err := WriteCollectionJSON(&out, summary); err != nil {
		t.Fatal(err)
	}
	want := "  \"hostname\": \"a<b>&c\\u2028d\ufffde\",\n"
	if !strings.Contains(out.String(), want) {
		t.Errorf("collection.json lacks %s:\n%s", want, out.String())
	}
}

func TestCollectionJSONCountsFollowTheStatusOrder(t *testing.T) {
	var out bytes.Buffer
	counts := map[manifest.Status]int{}
	for index, status := range manifest.Statuses() {
		counts[status] = index + 1
	}
	summary := sampleSummary()
	summary.Counts = manifest.Counts{ByStatus: counts, CollectedBytes: 99}
	if err := WriteCollectionJSON(&out, summary); err != nil {
		t.Fatal(err)
	}
	var want strings.Builder
	for index, status := range manifest.Statuses() {
		want.WriteString("    " + quoted(string(status)) + ": " + strconv.Itoa(index+1) + ",\n")
	}
	want.WriteString("    \"collected_bytes\": 99\n")
	if !strings.Contains(out.String(), want.String()) {
		t.Errorf("counts are not one per status in order, then the bytes:\n%s\nwant:\n%s",
			out.String(), want.String())
	}
}

func TestCollectionJSONKeepsUsersAndHomesInStepAndLeavesOutOtherEmptyItems(t *testing.T) {
	summary := sampleSummary()
	summary.Users = []string{"", "a"}
	summary.Homes = []string{"/h/nameless", "/h/a"}
	summary.Notes = []string{"", "n1"}
	summary.Projects = nil
	var out bytes.Buffer
	if err := WriteCollectionJSON(&out, summary); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"  \"users\": [\n    \"\",\n    \"a\"\n  ],\n",
		"  \"homes\": [\n    \"/h/nameless\",\n    \"/h/a\"\n  ],\n",
		"  \"projects\": [],\n", "  \"notes\": [\n    \"n1\"\n  ],\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("collection.json lacks %q:\n%s", want, out.String())
		}
	}
}

func TestCollectionJSONListsAnEmptyUserNameAmongTheUsersWithArtifacts(t *testing.T) {
	summary := sampleSummary()
	summary.Users = []string{"", "a"}
	summary.Homes = []string{"/h/nameless", "/h/a"}
	summary.ActiveUsers = []string{"", "a"}
	var out bytes.Buffer
	if err := WriteCollectionJSON(&out, summary); err != nil {
		t.Fatal(err)
	}
	want := "  \"users_with_artifacts\": [\n    \"\",\n    \"a\"\n  ],\n"
	if !strings.Contains(out.String(), want) {
		t.Errorf("collection.json lacks %q:\n%s", want, out.String())
	}
	var result strings.Builder
	if err := PrintResult(&result, summary); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.String(), "users:      2\n") {
		t.Errorf("summary does not count both users:\n%s", result.String())
	}
}

func TestWriteCollectionJSONRegistryUserLists(t *testing.T) {
	var out bytes.Buffer
	summary := sampleSummary()
	summary.Relocation = Relocation{RegistryRead: true,
		UsersRead: []string{"alice", `b"ob`}, UsersNotLoaded: []string{"carol"},
		UsersDenied: []string{"dave"}}
	if err := WriteCollectionJSON(&out, summary); err != nil {
		t.Fatal(err)
	}
	want := `    "hints": [],
    "registry": {
      "read": true,
      "reason": "",
      "users_read": [
        "alice",
        "b\"ob"
      ],
      "users_not_loaded": [
        "carol"
      ],
      "users_denied": [
        "dave"
      ]
    },
`
	if !strings.Contains(out.String(), want) {
		t.Errorf("relocation object:\n%s\nlacks:\n%s", out.String(), want)
	}
}

// resultCase is a summary and the stdout text it should print.
type resultCase struct {
	name    string
	summary Summary
	want    string
}

// resultCases is the table of TestPrintResult.
func resultCases() []resultCase {
	counts := manifest.Counts{
		ByStatus: map[manifest.Status]int{
			manifest.Collected: 1, manifest.SkippedExcluded: 2, manifest.ErrorCopy: 1,
			manifest.ErrorRead: 2,
		},
		CollectedBytes: 1,
	}
	base := Summary{
		ArchivePath: "/o/a.tar.gz", ArchiveSize: 2048, ArchiveSHA: "abc", ManifestPath: "/o/m",
		SummaryPath: "/o/s", LogPath: "/o/l", ActiveUsers: []string{"a"}, Projects: []string{"/p", "/q"},
		Counts: counts,
	}
	const head = `archive:    /o/a.tar.gz
size:       2048 (2.0 KiB)
sha256:     abc
manifest:   /o/m
summary:    /o/s
log:        /o/l
users:      1
projects:   2
collected:  1 file, 1 byte (1 B)
skipped:    2 excluded, 0 too large, 0 secret, 0 special
errors:     3 (1 error_copy, 2 error_read)
`
	withHints := base
	withHints.Relocation = Relocation{
		Hints: []Hint{{}}, FilesSearched: 1, FilesSkippedSize: 2, Resolved: 3,
	}
	withDocker := base
	withDocker.Docker = volumes.Counts{Seen: true, Found: 1, Unreadable: 1, Desktop: true}
	return []resultCase{
		{"no hints and no docker", base, head + "relocation: 0 hints in 0 files searched\n"},
		{"hints, skipped and resolved", withHints, head +
			"relocation: 1 hint in 1 file searched, 2 skipped for size, 3 resolved; " +
			"see docs/relocation.md\n"},
		{"docker with an unreadable volume and Docker Desktop", withDocker, head +
			"relocation: 0 hints in 0 files searched\n" +
			"docker:     1 volume found, 0 collected, 1 unreadable (run as root to collect); " +
			"Docker Desktop VM disk not collected\n"},
	}
}

func TestPrintResult(t *testing.T) {
	for _, test := range resultCases() {
		t.Run(test.name, func(t *testing.T) {
			var out strings.Builder
			if err := PrintResult(&out, test.summary); err != nil {
				t.Fatal(err)
			}
			if out.String() != test.want {
				t.Errorf("got:\n%s\nwant:\n%s", out.String(), test.want)
			}
		})
	}
}
