package openclaw

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// longValue is the length of each long value the fixtures hold: more than
// the bytes a value copied onto many rows is written with.
const longValue = 10000

// cutProblem returns the problem row text of a value called name cut once.
func cutProblem(name string) string {
	return parser.ProblemPrefix + parser.CarriedProblem(name, 1)
}

// parsedTexts returns the texts of the rows a parse of artifact gives.
func parsedTexts(t *testing.T, name string) []string {
	t.Helper()
	var texts []string
	if err := New().Parse(agentArtifact(t, name), parser.Options{}, func(row timeline.Row) {
		texts = append(texts, row.Text)
	}); err != nil {
		t.Fatal(err)
	}
	return texts
}

// problemsOf returns the texts that are `parser:` rows.
func problemsOf(texts []string) []string {
	var problems []string
	for _, text := range texts {
		if strings.HasPrefix(text, parser.ProblemPrefix) {
			problems = append(problems, text)
		}
	}
	return problems
}

func TestAWindowsValuesAreCutOnceForEveryGenerationThatCopiesThem(t *testing.T) {
	texts := parsedTexts(t, "window_archives.db")
	// A start row holds the key, the display name and the provider, each
	// cut, beside a few short words.
	const most = 3*parser.CarriedLimit + 256
	starts := 0
	for _, text := range texts {
		if len(text) > most {
			t.Errorf("a row of %d bytes, more than %d: %.80q", len(text), most, text)
		}
		if strings.HasPrefix(text, "session start:") {
			starts++
			if !strings.Contains(text, "key=kkkk") || !strings.Contains(text, " [cut from 10000 bytes]") {
				t.Errorf("start row %.120q does not hold the cut key", text)
			}
		}
	}
	if starts != 10 {
		t.Errorf("%d start rows, want one per generation (10)", starts)
	}
	want := []string{cutProblem("session_windows.session_key"), cutProblem("session_windows.display_name"),
		cutProblem("session_windows.model_provider"), cutProblem("session_windows.model")}
	if got := problemsOf(texts); !slices.Equal(got, want) {
		t.Errorf("problems %q, want %q", got, want)
	}
}

func TestEachCopyOfALegacyTranscriptCutsItsLongSessionKeyOnce(t *testing.T) {
	home := t.TempDir()
	key := strings.Repeat("k", longValue)
	writeHomeFile(t, home, ".openclaw/sessions/sessions.json", `{"`+key+`": {"sessionId": "s"}}`)
	files := []string{".openclaw/sessions/s.jsonl"}
	for i := range 3 {
		files = append(files, ".openclaw/sessions/s.checkpoint.00000000-0000-0000-0000-00000000000"+
			strconv.Itoa(i)+".jsonl")
	}
	subject := New()
	for _, rel := range files {
		writeHomeFile(t, home, rel, `{"type": "label", "label": "l"}`+"\n")
		texts := parseHomeFile(t, subject, home, rel)
		for _, text := range texts {
			if most := parser.CarriedLimit + 256; len(text) > most {
				t.Errorf("%s: a row of %d bytes, more than %d: %.80q", rel, len(text), most, text)
			}
		}
		if got, want := problemsOf(texts), []string{cutProblem("session key")}; !slices.Equal(got, want) {
			t.Errorf("%s: problems %q, want %q", rel, got, want)
		}
	}
}
