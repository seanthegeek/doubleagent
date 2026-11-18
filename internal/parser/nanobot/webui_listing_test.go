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
