//go:build unix

package agentzero

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

func TestAMessageFileIsNeverReadThroughALinkedMessagesDirectory(t *testing.T) {
	tests := []struct {
		name, want string
		linked     bool
	}{
		{"a messages directory beside the chat is read", "kept", false},
		{"a link to a directory outside the home reads nothing", "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			const rel = "usr/chats/ctx/chat.json"
			writeFile(t, home, rel, "{}")
			if test.linked {
				outside := t.TempDir()
				writeFile(t, outside, "1.txt", "kept")
				if err := os.Symlink(outside, filepath.Join(home, "usr", "chats", "ctx", "messages")); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFile(t, home, "usr/chats/ctx/messages/1.txt", "kept")
			}
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Rel: rel,
				Home: &input.Home{DiskPath: home}}
			state := &chat{messages: messagesBeside(artifact)}
			content := jsonvalue.NewObject(jsonvalue.Member{Key: "file", Value: jsonvalue.NewString("1.txt")})
			if got := state.messageFile(content); got != test.want || state.failure != nil {
				t.Errorf("got %q, failure %v; want %q", got, state.failure, test.want)
			}
		})
	}
}
