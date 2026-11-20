package parsers

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The Muse Code growth cases: the session logs and their subagent logs,
// the prompt history and trust.json.

const museSession = ".local/share/muse/sessions/2026/10/01/01a0f000-0000-7000-8000-000000000001/"

// museRecord returns an envelope of the session stream with the payload.
func museRecord(kind, payload string) string {
	return `{"stream":{"id":"s1"},"recorded_at":1790000000000000,"payload_type":"` + kind + `","payload":` +
		payload + `}`
}

// museBare returns an envelope with no stream, which takes the stream of
// the records before it.
func museBare(kind, payload string) string {
	return `{"recorded_at":1790000000000000,"payload_type":"` + kind + `","payload":` + payload + `}`
}

// museRun returns a runtime.session record of a run event.
func museRun(event string) string {
	return museRecord("runtime.session", `{"kind":"run","run_id":"r1","event":`+event+`}`)
}

// museName returns a session name record.
func museName(i int) string {
	return museBare("session.name.changed", `{"record":{"new_name":"n`+strconv.Itoa(i)+`"}}`)
}

// musecodeGrowth returns the muse-code growth set.
func musecodeGrowth() growthSet {
	return growthSet{parser: "muse-code", cases: slices.Concat(museSessionCases(), museShapeCases(), museFileCases())}
}

// museSessionCases returns the cases of the session logs where one long
// value stands in many rows.
func museSessionCases() []growthCase {
	log := museSession + "session.jsonl"
	return []growthCase{
		{kind: "session log", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			first := `{"stream":{"id":"` + long("stream") + `"},"recorded_at":1790000000000000,"payload_type":` +
				`"runtime.session.metadata","payload":{"record":{"workspace_root":"/` + long("root") +
				`","model_id":"` + long("model") + `"}}}` + "\n" +
				museBare("session.workspace_branch.observed", `{"record":{"reference":{"kind":"branch","name":"`+
					long("branch")+`"}}}`) + "\n"
			return one(log, append([]byte(first), lines(n, museName)...))
		}},
		{kind: "session log", shape: shapeCarried + " in the permission format",
			files: func(t *testing.T, n int) []growthFile {
				first := museRecord("runtime.session.permission_format_declared", `{"record":{"format":"`+long("format")+`"}}`)
				return one(log, append([]byte(first+"\n"), lines(n, func(i int) string {
					return museRecord("runtime.session.permission_profile_committed",
						`{"record":{"resolved_snapshot":{"approval":"a`+strconv.Itoa(i)+`"}}}`)
				})...))
			}},
		{kind: "session log", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			first := museRecord("runtime.session", `{"kind":"approval","event":{"kind":"requested","tool_name":"`+
				long("tool")+`","tool_call_id":"c1","pending_action_id":"p1","raw_args":"x"}}`)
			return one(log, append([]byte(first+"\n"), lines(n, func(i int) string {
				return museRecord("runtime.session", `{"kind":"approval","event":{"kind":"decision_applied",`+
					`"pending_action_id":"p1","decision":"allow"}}`)
			})...))
		}},
	}
}

// museShapeCases returns the cases of the session logs that repeat ids,
// nest and damage records, and spread a session over many subagent logs.
func museShapeCases() []growthCase {
	log := museSession + "session.jsonl"
	return []growthCase{
		{kind: "session log", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(log, lines(n, func(i int) string {
				return museRun(`{"kind":"assistant_tool_calls_committed","tool_calls":[{"name":"bash","call_id":"c1",`+
					`"args":{"command":"ls"}}]}`) + "\n" +
					museRun(`{"kind":"tool_result_committed","tool_call_id":"c1","output":"r"}`) + "\n" +
					museRun(`{"kind":"reasoning_summary_delta","message_id":"m1","text":"d`+strconv.Itoa(i)+`"}`)
			}))
		}},
		{kind: "session log", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			children := joined(n, func(i int) string {
				if i%2 == 0 {
					return `{"child_index":` + strconv.Itoa(n-i) + `,"record_json":"{\"cut"}`
				}
				return `{"child_index":` + strconv.Itoa(n-i) + `,"record_json":` + strconv.Quote(museName(i)) + `}`
			})
			deep := museRun(`{"kind":"run_retracted","detail":` + nestedArrays(n, `"x"`) + `}`)
			return one(log, []byte(museRecord("x", "{}")+"\n"+`{"retained_frame":1,"children":[`+children+"]}\n"+
				deep+"\n"))
		}},
		{kind: "session log", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			parent := museRecord("runtime.session.metadata", `{"record":{"workspace_root":"/w","model_id":"m"}}`)
			return append(manyFiles(n, func(i int) string {
				return museSession + "subagent/" + numbered("child", i) + "/session.jsonl"
			}, func(i int) string {
				return museRun(`{"kind":"started","prompt":"task `+strconv.Itoa(i)+`"}`) + "\n"
			}), text(log, parent+"\n"+strings.Repeat(museName(0)+"\n", n)))
		}},
	}
}

// museFileCases returns the cases of the prompt history and trust.json.
func museFileCases() []growthCase {
	return []growthCase{
		{kind: "tui-history.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".local/share/muse/tui-history.jsonl", lines(n, func(i int) string {
				return `"prompt ` + strconv.Itoa(i) + `"` + "\n" + `{"project":"/p","session":"s"}` + "\n" + `[1,` +
					"\n" + `{"cut`
			}))
		}},
		{kind: "trust.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".config/muse/trust.json", []byte(`{"projects":{`+joined(n, func(i int) string {
				return `"/p` + strconv.Itoa(i) + `":` + strconv.Itoa(i) + `,"/q` + strconv.Itoa(i) + `":{"decision":"trusted"}`
			})+`,"/deep":{"decision":`+nestedArrays(n, `"x"`)+`}}}`))
		}},
	}
}
