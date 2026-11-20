package nanobot

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// Each chat of a WebUI transcript looks for its session file in every
// session namespace, so each namespace is listed once for the file and the
// chat's file name looked up in the listing, not looked at once per chat
// and namespace.
func TestEachSessionNamespaceIsListedOnceForAllChatsOfAFile(t *testing.T) {
	const namespaces = 5
	for _, chats := range []int{4, 40} {
		t.Run(fmt.Sprintf("%d chats over %d namespaces make %d listings", chats, namespaces, namespaces),
			func(t *testing.T) {
				home := t.TempDir()
				files := map[string]string{}
				var records []string
				for i := range chats {
					name := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("websocket:chat-%d", i))) + ".jsonl"
					files[fmt.Sprintf(".nanobot/sessions/ns%d/%s", i%namespaces, name)] = ""
					records = append(records, fmt.Sprintf(`{"event": "user", "chat_id": "chat-%d", "text": "hi"}`, i))
				}
				for i := range namespaces {
					files[fmt.Sprintf(".nanobot/sessions/ns%d/.workspace", i)] = fmt.Sprintf("/work/p%d\n", i)
				}
				files[".nanobot/webui/w.jsonl"] = strings.Join(records, "\n") + "\n"
				for rel, text := range files {
					path := filepath.Join(home, filepath.FromSlash(rel))
					if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
						t.Fatal(err)
					}
					if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				const rel = ".nanobot/webui/w.jsonl"
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
					Home: &input.Home{DiskPath: home}, Manifest: jsonvalue.NewNull()}
				subject := New()
				subject.Work = parser.CountingWork()
				rows := 0
				err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
					var chat int
					if _, err := fmt.Sscanf(row.SessionID, "websocket:chat-%d", &chat); err != nil {
						t.Fatalf("row %q has session %q", row.Text, row.SessionID)
					}
					if want := fmt.Sprintf("/work/p%d", chat%namespaces); row.ProjectPath != want {
						t.Errorf("chat %d has project %q, want %q", chat, row.ProjectPath, want)
					}
					rows++
				})
				if err != nil || rows != chats {
					t.Fatalf("got %d rows and %v, want %d rows", rows, err, chats)
				}
				if listings := subject.Work.Count("namespace listings"); listings != namespaces {
					t.Errorf("%d chats made %d namespace listings, want %d", chats, listings, namespaces)
				}
			})
	}
}

// writeHome writes files, by their path relative to a new home, and
// returns the home.
func writeHome(t *testing.T, files map[string]string) string {
	t.Helper()
	home := t.TempDir()
	for rel, text := range files {
		path := filepath.Join(home, filepath.FromSlash(rel))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

// Every WebUI file of a run looks for its chats' session files in the same
// session namespaces, so each namespace is listed once for the run and a
// chat looks only in the namespaces whose listing holds its file: the work
// grows with the files and the namespaces, not with their product.
func TestEachSessionNamespaceIsListedOnceForAllWebUIFilesOfARun(t *testing.T) {
	for _, n := range []int{20, 40} {
		t.Run(fmt.Sprintf("%d files over %d namespaces", n, n), func(t *testing.T) {
			files := map[string]string{}
			for i := range n {
				name := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("websocket:chat-%d", i))) + ".jsonl"
				files[fmt.Sprintf(".nanobot/sessions/ns%03d/%s", i, name)] = ""
				files[fmt.Sprintf(".nanobot/sessions/ns%03d/.workspace", i)] = fmt.Sprintf("/work/p%d\n", i)
				files[fmt.Sprintf(".nanobot/webui/w%03d.jsonl", i)] =
					fmt.Sprintf(`{"event": "user", "chat_id": "chat-%d", "text": "hi"}`, i) + "\n"
			}
			home := writeHome(t, files)
			subject := New()
			subject.Work = parser.CountingWork()
			for i := range n {
				rel := fmt.Sprintf(".nanobot/webui/w%03d.jsonl", i)
				artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
					Home: &input.Home{DiskPath: home}, Manifest: jsonvalue.NewNull()}
				err := subject.Parse(artifact, parser.Options{}, func(row timeline.Row) {
					if want := fmt.Sprintf("/work/p%d", i); row.ProjectPath != want {
						t.Errorf("file %d has project %q, want %q", i, row.ProjectPath, want)
					}
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if listings, lookups := subject.Work.Count("namespace listings"),
				subject.Work.Count("namespace lookups"); listings != n || lookups != n {
				t.Errorf("%d files made %d namespace listings and %d lookups, want %d of each",
					n, listings, lookups, n)
			}
		})
	}
}
