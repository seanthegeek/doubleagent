package timeline

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"strconv"
	"testing"
)

// mixedBatch is one batch of testdata/sessions.json: rows whose fields come
// from small pools, so sessions, sources, times and marks collide, the agents
// of each mark, the sessions.jsonl lines of their summaries and the indices
// of the rows in timeline order.
type mixedBatch struct {
	Name  string `json:"name"`
	Marks struct {
		Synced    []string `json:"synced"`
		Relayed   []string `json:"relayed"`
		Origin    []string `json:"origin"`
		Secondary []string `json:"secondary"`
	} `json:"marks"`
	Rows     []Row             `json:"rows"`
	Sessions []json.RawMessage `json:"sessions"`
	Order    []int             `json:"order"`
}

// readTestData decodes the named file under testdata into value.
func readTestData(t *testing.T, name string, value any) {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatal(err)
	}
}

// agentSet returns the set of the agents named.
func agentSet(agents []string) map[string]bool {
	set := map[string]bool{}
	for _, agent := range agents {
		set[agent] = true
	}
	return set
}

func TestMixedBatchesSummariseAndSort(t *testing.T) {
	var batches []mixedBatch
	readTestData(t, "sessions.json", &batches)
	for _, test := range batches {
		t.Run(test.Name, func(t *testing.T) {
			marks := MarkAgents{Synced: agentSet(test.Marks.Synced),
				Relayed: agentSet(test.Marks.Relayed), Origin: agentSet(test.Marks.Origin),
				Secondary: agentSet(test.Marks.Secondary)}
			var out, want bytes.Buffer
			if err := WriteSessions(&out, Summarise(test.Rows, marks)); err != nil {
				t.Fatal(err)
			}
			for _, line := range test.Sessions {
				if err := json.Compact(&want, line); err != nil {
					t.Fatal(err)
				}
				want.WriteString("\n")
			}
			if out.String() != want.String() {
				t.Errorf("sessions =\n%s\nwant\n%s", out.String(), want.String())
			}
			indexed := slices.Clone(test.Rows)
			for i := range indexed {
				indexed[i].ToolUseID = strconv.Itoa(i)
			}
			SortRows(indexed)
			order := make([]int, len(indexed))
			for i, row := range indexed {
				order[i], _ = strconv.Atoi(row.ToolUseID)
			}
			if !slices.Equal(order, test.Order) {
				t.Errorf("order = %v, want %v", order, test.Order)
			}
		})
	}
}

func TestMixedPathSpellings(t *testing.T) {
	var paths []struct {
		Path      string `json:"path"`
		Canonical string `json:"canonical"`
		Key       string `json:"key"`
		Windows   bool   `json:"windows"`
	}
	readTestData(t, "paths.json", &paths)
	for _, test := range paths {
		t.Run(strconv.Quote(test.Path), func(t *testing.T) {
			if got := CanonicalProjectPath(test.Path); got != test.Canonical {
				t.Errorf("CanonicalProjectPath = %q, want %q", got, test.Canonical)
			}
			if got := ProjectKey(test.Path); got != test.Key {
				t.Errorf("ProjectKey = %q, want %q", got, test.Key)
			}
			if got := IsWindowsPath(test.Path); got != test.Windows {
				t.Errorf("IsWindowsPath = %v, want %v", got, test.Windows)
			}
		})
	}
}

func FuzzProjectPathSpelling(fuzz *testing.F) {
	for _, test := range pathCases() {
		fuzz.Add(test.input)
	}
	fuzz.Fuzz(func(t *testing.T, path string) {
		canonical := CanonicalProjectPath(path)
		if again := CanonicalProjectPath(canonical); again != canonical {
			t.Fatalf("CanonicalProjectPath(%q) = %q, but of that %q", path, canonical, again)
		}
		if IsWindowsPath(path) != IsWindowsPath(canonical) {
			t.Fatalf("IsWindowsPath differs between %q and its canonical form %q", path, canonical)
		}
		if !IsWindowsPath(path) && (canonical != path || ProjectKey(path) != path) {
			t.Fatalf("a path in no Windows form changed: %q, %q", canonical, ProjectKey(path))
		}
		if key := ProjectKey(path); ProjectKey(key) != key {
			t.Fatalf("ProjectKey(%q) = %q, but of that %q", path, key, ProjectKey(key))
		}
	})
}
