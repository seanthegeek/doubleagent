package growthtests

import (
	"strconv"
	"strings"
	"testing"
)

// The little-coder growth cases: the checkpoint sets and the pi session
// files they look up, the prompt and input histories, the v0.0.x config,
// saved sessions and snapshots.

// littlecoderSession returns the name of pi session file i, which its
// checkpoint set is named after.
func littlecoderSession(i int) string {
	return "2026-01-01T00-00-00_" + numbered("s", i) + ".jsonl"
}

// littlecoderMessages returns n v0.0.x messages: a user prompt, then an
// assistant reply calling a tool, then the tool's denied result, in turn.
func littlecoderMessages(n int) string {
	return joined(n, func(i int) string {
		switch i % 3 {
		case 0:
			return `{"role":"user","content":"` + numbered("prompt ", i) + `"}`
		case 1:
			return `{"role":"assistant","content":"ok","tool_calls":[{"id":"` + numbered("c", i) +
				`","name":"Bash","input":{"command":"ls"}}]}`
		}
		return `{"role":"tool","name":"Bash","tool_call_id":"` + numbered("c", i-1) +
			`","content":"Denied: user rejected this operation"}`
	})
}

// littlecoderCheckpointCases returns the cases of the checkpoint sets: many
// sets each looking for its session among many project directories, one
// set of many files, and v0.0.x snapshots sharing one id.
func littlecoderCheckpointCases() []growthCase {
	return []growthCase{
		{kind: "checkpoint", shape: shapeManyFiles + " (sets looking among the project directories)", n: 16,
			files: func(t *testing.T, n int) []growthFile {
				var files []growthFile
				for i := range n {
					files = append(files, text(".little-coder/checkpoints/"+littlecoderSession(i)+"/a.py", "x"),
						text(".pi/agent/sessions/"+numbered("--p", i)+"--/"+littlecoderSession(i),
							piHeader(numbered("s", i), numbered("/p", i), "")+"\n"))
				}
				return files
			}},
		{kind: "checkpoint", shape: shapeUnlisted + " (pi project directories)", n: 16,
			files: func(t *testing.T, n int) []growthFile {
				var files []growthFile
				for i := range n {
					files = append(files, text(".little-coder/checkpoints/"+littlecoderSession(i)+"/a.py", "x"),
						text(".pi/agent/sessions/"+numbered("--p", i)+"--/"+littlecoderSession(i),
							piHeader(numbered("s", i), numbered("/p", i), "")+"\n"))
				}
				return files
			}, unlisted: func(n int) []string {
				rels := make([]string, 0, n)
				for i := range n {
					rels = append(rels, ".pi/agent/sessions/"+numbered("--p", i)+"--")
				}
				return rels
			}},
		{kind: "checkpoint", shape: shapeManyFiles + " (one set of many files)", n: 32,
			files: func(t *testing.T, n int) []growthFile {
				return append(manyFiles(n, func(i int) string {
					return ".little-coder/checkpoints/" + littlecoderSession(0) + "/" + numbered("f", i) + ".py"
				}, func(i int) string { return "the file as it was before the session first edited it\n" }),
					text(".pi/agent/sessions/--p--/"+littlecoderSession(0),
						piHeader("s0", "/p", "")+"\n"))
			}},
		{kind: "checkpoint", shape: shapeSharedID + " (v0.0.x snapshots)", files: func(t *testing.T, n int) []growthFile {
			return one(".little-coder/checkpoints/s1/snapshots.json", []byte("["+joined(n, func(i int) string {
				return `{"id":"same","session_id":"s1","turn_count":1,"user_prompt_preview":"fix",` +
					`"created_at":"2026-01-01T00:00:00","file_backups":{"a.py":{"backup_filename":"a.py.bak"},"b.py":null}}`
			})+"]"))
		}},
		{kind: "checkpoint", shape: shapeNested + " (v0.0.x snapshots)", files: func(t *testing.T, n int) []growthFile {
			return one(".little-coder/checkpoints/s1/snapshots.json", []byte("["+joined(n, strconv.Itoa)+
				`,{"id":"deep","file_backups":{"a.py":7,"b.py":`+nestedArrays(n, "1")+`}}]`))
		}},
	}
}

// littlecoderHistoryCases returns the cases of the prompt history, the
// v0.0.x input history and the v0.0.x config.
func littlecoderHistoryCases() []growthCase {
	return []growthCase{
		{kind: "prompt history", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".pi/agent/little-coder-prompt-history.json", []byte("["+joined(n, func(i int) string {
				return `"the same prompt"`
			})+"]"))
		}},
		{kind: "prompt history", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".pi/agent/little-coder-prompt-history.json", []byte("["+joined(n, strconv.Itoa)+","+
				nestedArrays(n, `"x"`)+`,"last"]`))
		}},
		{kind: "input_history.txt", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".little-coder/input_history.txt", lines(n, func(i int) string { return "the same prompt" }))
		}},
		{kind: "input_history.txt", shape: shapeNested + " (lines not UTF-8)", files: func(t *testing.T, n int) []growthFile {
			return one(".little-coder/input_history.txt", lines(n, func(i int) string {
				return "bad \xff\xfe " + strconv.Itoa(i)
			}))
		}},
		{kind: "config.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(".little-coder/config.json", []byte(`{"permission_mode":`+nestedArrays(n, `"auto"`)+","+
				joined(n, func(i int) string { return `"` + numbered("p", i) + `_api_key":"` + numbered("sk-", i) + `"` })+
				`,"telegram_chat_id":0,"cloudsave_last_gist_id":"g1"}`))
		}},
	}
}

// littlecoderLegacyCases returns the cases of the v0.0.x saved sessions:
// a long session id every message's row carries, whole and cut mid-write,
// the history's sessions sharing one id, and damaged messages.
func littlecoderLegacyCases() []growthCase {
	saved := func(identifier, messages string) string {
		return `{"session_id":"` + identifier + `","saved_at":"2026-01-01T00:00:00","turn_count":3,"messages":[` + messages
	}
	return []growthCase{
		{kind: "legacy session", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".little-coder/sessions/2026-01-01/a.json", []byte(saved(long("s"), littlecoderMessages(n))+"]}"))
		}},
		{kind: "legacy session", shape: shapeCarried + " (a file cut mid-write)", files: func(t *testing.T, n int) []growthFile {
			return one(".little-coder/sessions/2026-01-01/a.json", []byte(saved(long("s"), littlecoderMessages(n))+`,{"ro`))
		}},
		{kind: "legacy session", shape: shapeSharedID + " (sessions of history.json)",
			files: func(t *testing.T, n int) []growthFile {
				return one(".little-coder/sessions/history.json", []byte(`{"sessions":[`+joined(n, func(i int) string {
					return saved("same", littlecoderMessages(2)) + "]}"
				})+"]}"))
			}},
		{kind: "legacy session", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(".little-coder/sessions/mr_sessions/session_latest.json", []byte(saved("s1",
				strings.Repeat(`7,`, n)+`{"role":"assistant","content":"x","tool_calls":[`+joined(n, strconv.Itoa)+
					`]},{"role":"user","content":`+nestedArrays(n, `"x"`)+`}`)+"]}"))
		}},
	}
}

// littlecoderGrowth returns the little-coder growth set.
func littlecoderGrowth() growthSet {
	var cases []growthCase
	for _, part := range [][]growthCase{littlecoderCheckpointCases(), littlecoderHistoryCases(),
		littlecoderLegacyCases()} {
		cases = append(cases, part...)
	}
	return growthSet{parser: "little-coder", cases: cases}
}
