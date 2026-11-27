package musecode

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseFile writes text at rel under a new home and returns the texts of
// the rows the parser gives it.
func parseFile(t *testing.T, rel, text string) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Agent: "musecode",
		Home: &input.Home{DiskPath: home}, Manifest: jsonvalue.NewNull()}
	var texts []string
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

// countOf returns how many of the rows have exactly the given text.
func countOf(rows []string, text string) int {
	total := 0
	for _, row := range rows {
		if row == text {
			total++
		}
	}
	return total
}

// checkMember parses a file built around each value of a member and checks
// that the problem row appears once for a string, a number and a value of
// the other container kind, and not for null, an empty value or an absent
// member (an empty text for the value).
func checkMember(t *testing.T, rel, member string, kind jsonvalue.Kind, other string, build func(value string) string) {
	t.Helper()
	problem := parserkit.ProblemPrefix + parserkit.WrongKind(member, kind)
	empty := `[]`
	if kind == jsonvalue.Object {
		empty = `{}`
	}
	tests := []struct {
		name, json string
		want       int
	}{
		{"a string", `"oops"`, 1}, {"a number", `7`, 1}, {"the other kind", other, 1},
		{"null", `null`, 0}, {"empty", empty, 0}, {"absent", "", 0},
	}
	for _, test := range tests {
		t.Run(member+" "+test.name, func(t *testing.T) {
			if got := countOf(parseFile(t, rel, build(test.json)), problem); got != test.want {
				t.Errorf("%d rows %q, want %d", got, problem, test.want)
			}
		})
	}
}

// field returns `"name": value,` or nothing for no value.
func field(name, value string) string {
	if value == "" {
		return ""
	}
	return `"` + name + `": ` + value + `,`
}

const sessionRel = ".local/share/muse/sessions/2026/10/01/01a0f000-0000-7000-8000-000000000001/session.jsonl"

// sessionLog returns a session log of one envelope with the payload type
// and the payload's JSON.
func sessionLog(payloadType, payload string) string {
	return `{"stream": {"id": "s"}, "payload_type": "` + payloadType + `", "payload": ` + payload + `}` + "\n"
}

func TestAFrameChildrenThatIsNotAnArrayIsReported(t *testing.T) {
	checkMember(t, sessionRel, "children", jsonvalue.Array, `{"child_index": 0}`, func(value string) string {
		return `{` + field("children", value) + `"retained_frame": "session_permission_transaction"}` + "\n"
	})
}

func TestAFrameChildThatDecodesToAnythingButAnObjectOrNullIsReported(t *testing.T) {
	const wrong = "parser: unparseable line (retained_frame child: record_json is not a JSON object): "
	for _, test := range []struct {
		name, record string
		want         []string
	}{
		{"a child that decodes to null gives no row", "null", nil},
		{"a child that decodes to a number", "7", []string{wrong + "7"}},
		{"a child that decodes to a list", "[1]", []string{wrong + "[1]"}},
		{"a child that decodes to a string", `"s"`, []string{wrong + `"s"`}},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.record)
			if err != nil {
				t.Fatal(err)
			}
			log := `{"retained_frame": {}, "children": [{"child_index": 0, "record_json": ` + string(encoded) + `}]}` + "\n"
			if got := parseFile(t, sessionRel, log); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestAUserIntentMemberThatIsNotAnArrayIsReported(t *testing.T) {
	checkMember(t, sessionRel, "model_messages", jsonvalue.Array, `{"content": []}`, func(value string) string {
		return sessionLog("runtime.user_intent.accepted", `{`+field("model_messages", value)+`"intent_id": "i"}`)
	})
	checkMember(t, sessionRel, "model_messages.content", jsonvalue.Array, `{"text": "x"}`, func(value string) string {
		return sessionLog("runtime.user_intent.accepted",
			`{"model_messages": [{`+field("content", value)+`"role": "user"}], "intent_id": "i"}`)
	})
}

func TestACommittedToolCallsThatIsNotAnArrayIsReported(t *testing.T) {
	checkMember(t, sessionRel, "tool_calls", jsonvalue.Array, `{"name": "bash"}`, func(value string) string {
		return sessionLog("runtime.session", `{"kind": "run", "run_id": "r", "event": {`+
			field("tool_calls", value)+`"kind": "assistant_tool_calls_committed"}}`)
	})
}

func TestATrustProjectsThatIsNotAnObjectIsReported(t *testing.T) {
	checkMember(t, ".config/muse/trust.json", "projects", jsonvalue.Object, `[1]`, func(value string) string {
		return `{` + field("projects", value) + `"version": 1}`
	})
}

func TestEntriesThatAreNotObjectsAreReportedOncePerList(t *testing.T) {
	tests := []struct {
		name, rel, text, want string
	}{
		{"user intent messages", sessionRel,
			sessionLog("runtime.user_intent.accepted", `{"model_messages": [{"content": [{"text": "hi"}]}, 7], "intent_id": "i"}`),
			"parser: model_messages holds 1 entry that is not a JSON object"},
		{"a message's content", sessionRel,
			sessionLog("runtime.user_intent.accepted", `{"model_messages": [{"content": [{"text": "hi"}, "x"]}], "intent_id": "i"}`),
			"parser: model_messages.content holds 1 entry that is not a JSON object"},
		{"trust projects", ".config/muse/trust.json", `{"version": 1, "projects": {"/w": {"decision": "trust"}, "/x": 7}}`,
			"parser: projects holds 1 entry that is not a JSON object"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := countOf(parseFile(t, test.rel, test.text), test.want); got != 1 {
				t.Errorf("%q given %d times, want once", test.want, got)
			}
		})
	}
}

func TestAMessageContentOfTheWrongKindInManyMessagesIsOneProblem(t *testing.T) {
	tests := []struct {
		name, messages, want string
	}{
		{"contents that are not arrays", `[{"content": 7}, {"content": "x"}, {"content": {}}]`,
			"parser: model_messages.content is not a JSON array in 3 entries"},
		{"contents holding entries that are not objects", `[{"content": [7]}, {"content": ["x", 8]}]`,
			"parser: model_messages.content holds 3 entries that are not JSON objects"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := parseFile(t, sessionRel, sessionLog("runtime.user_intent.accepted",
				`{"model_messages": `+test.messages+`, "intent_id": "i"}`))
			var problems []string
			for _, row := range rows {
				if strings.HasPrefix(row, "parser: ") {
					problems = append(problems, row)
				}
			}
			if !slices.Equal(problems, []string{test.want}) {
				t.Errorf("problems %q, want %q", problems, test.want)
			}
		})
	}
}
