package growthtests

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"testing"
)

// The OpenHands growth cases: a conversation's event files and the
// meta.json and base_state.json each of them looks up, the CLI's prompt
// histories that look up every conversation's working directory, and the
// legacy sessions.

// openhandsConversation returns the directory of conversation i.
func openhandsConversation(i int) string {
	return fmt.Sprintf(".openhands/conversations/%032x/", i)
}

// openhandsEvent returns the path of event i of a conversation directory.
func openhandsEvent(conversation string, i int) string {
	return conversation + fmt.Sprintf("events/event-%05d-%08x.json", i, i)
}

// openhandsMessage returns a user MessageEvent at second i, with the extra
// members (each starting with a comma) after its own.
func openhandsMessage(i int, extra string) string {
	return `{"kind":"MessageEvent","id":"` + numbered("m", i) + `","timestamp":"2026-01-01T00:00:` +
		fmt.Sprintf("%02d", i%60) + `.000000","source":"user","llm_message":{"role":"user",` +
		`"content":[{"type":"text","text":"hi"}]}` + extra + `}`
}

// openhandsAction returns an ActionEvent of the given id and tool name
// running ls.
func openhandsAction(identifier, tool string) string {
	return `{"kind":"ActionEvent","id":"` + identifier + `","tool_name":"` + tool + `","tool_call_id":"c` + identifier +
		`","timestamp":"2026-01-01T00:00:00.000000","action":{"command":"ls"}}`
}

// openhandsEvents returns n event files of a conversation, event i
// holding event(i).
func openhandsEvents(conversation string, n int, event func(i int) string) []growthFile {
	return manyFiles(n, func(i int) string { return openhandsEvent(conversation, i) }, event)
}

// openhandsMeta returns a meta.json of a conversation working in a
// directory.
func openhandsMeta(conversation, workingDirectory string) growthFile {
	return text(conversation+"meta.json", `{"title":"fix the bug","created_at":"2026-01-01T00:00:00Z",`+
		`"workspace":{"working_dir":"`+workingDirectory+`"}}`)
}

// openhandsState returns the base_state.json of a CLI conversation
// working in a directory, where the CLI keeps it.
func openhandsState(conversation, workingDirectory string) growthFile {
	return text(conversation+"base_state.json", `{"agent":{"llm":{"model":"m"}},"workspace":{"working_dir":"`+
		workingDirectory+`"}}`)
}

// openhandsHistory returns the path of the CLI's prompt history of a
// working directory, named by its SHA-256.
func openhandsHistory(workingDirectory string) string {
	sum := sha256.Sum256([]byte(workingDirectory))
	return ".openhands/projects/" + hex.EncodeToString(sum[:]) + "/prompt_history.json"
}

// openhandsGrowth returns the openhands growth set.
func openhandsGrowth() growthSet {
	var cases []growthCase
	for _, part := range [][]growthCase{openhandsEventCases(), openhandsModelCases(), openhandsStateCases(),
		openhandsHistoryCases()} {
		cases = append(cases, part...)
	}
	return growthSet{parser: "openhands", cases: cases}
}

// openhandsEventCases returns the cases of the event files: a working
// directory every event inherits, actions sharing one id, many
// conversations, and damaged events.
func openhandsEventCases() []growthCase {
	conversation := openhandsConversation(1)
	return []growthCase{
		{kind: "event", shape: shapeCarried + " (meta.json's working directory)", n: 32,
			files: func(t *testing.T, n int) []growthFile {
				return append(openhandsEvents(conversation, n, func(i int) string { return openhandsMessage(i, "") }),
					openhandsMeta(conversation, "/"+long("w")))
			}},
		{kind: "event", shape: shapeSharedID, n: 32, files: func(t *testing.T, n int) []growthFile {
			return append(openhandsEvents(conversation, n, func(i int) string { return openhandsAction("same", "bash") }),
				text(conversation+"base_state.json", `{"agent":{"llm":{"model":"m"}},"blocked_actions":{"same":"hook"}}`),
				text(openhandsEvent(conversation, n), `{"kind":"SecurityAnalysisEvent","analyzer":"a",`+
					`"policy":"p","risks":{"same":"HIGH"}}`))
		}},
		{kind: "event", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			var files []growthFile
			for k := range n {
				directory := openhandsConversation(k)
				files = append(files, openhandsMeta(directory, numbered("/w", k)),
					text(directory+"base_state.json", `{"agent":{"llm":{"model":"m"}}}`),
					text(openhandsEvent(directory, 0), openhandsMessage(0, "")))
			}
			return files
		}},
		{kind: "event", shape: shapeNested, n: 32, files: func(t *testing.T, n int) []growthFile {
			files := openhandsEvents(conversation, n, func(i int) string { return `{"kind":` })
			return append(files, text(openhandsEvent(conversation, n), `{"kind":"Other","deep":`+
				nestedArrays(n, `"x"`)+`}`))
		}},
	}
}

// openhandsUsages returns a base_state.json whose token usages give the
// response ids their models.
func openhandsUsages(models map[string]string) string {
	usages := ""
	for _, identifier := range []string{"r1", "r2"} {
		if model, ok := models[identifier]; ok {
			if usages != "" {
				usages += ","
			}
			usages += `{"response_id":"` + identifier + `","model":"` + model + `"}`
		}
	}
	return `{"agent":{"llm":{"model":"base"}},"stats":{"usage_to_metrics":{"agent":{"token_usages":[` + usages + `]}}}}`
}

// openhandsModelCases returns the cases of the models the events take
// from base_state.json's token usages: one long model every event names,
// and two long models the events alternate between.
func openhandsModelCases() []growthCase {
	conversation := openhandsConversation(1)
	return []growthCase{
		{kind: "event", shape: shapeLargeShared + " (a response's model)", n: 32,
			files: func(t *testing.T, n int) []growthFile {
				return append(openhandsEvents(conversation, n, func(i int) string {
					return openhandsMessage(i, `,"llm_response_id":"r1"`)
				}), text(conversation+"base_state.json", openhandsUsages(map[string]string{"r1": long("a")})))
			}},
		{kind: "event", shape: shapeLargeShared + " (security verdicts on one action)", n: 32,
			files: func(t *testing.T, n int) []growthFile {
				files := openhandsEvents(conversation, n, func(i int) string {
					return `{"kind":"SecurityAnalysisEvent","analyzer":"a","policy":"p","risks":{"a":"HIGH"},` +
						`"timestamp":"2026-01-01T00:00:01.000000"}`
				})
				return append(files, text(openhandsEvent(conversation, n), openhandsAction("a", long("tool"))))
			}},
		{kind: "event", shape: shapeLargeShared + " (two responses' models in turn)", n: 32,
			files: func(t *testing.T, n int) []growthFile {
				return append(openhandsEvents(conversation, n, func(i int) string {
					return openhandsMessage(i, `,"llm_response_id":"r`+strconv.Itoa(1+i%2)+`"`)
				}), text(conversation+"base_state.json", openhandsUsages(map[string]string{"r1": long("a"),
					"r2": long("b")})))
			}},
	}
}

// openhandsStateCases returns the cases of base_state.json and meta.json:
// blocked actions joined to the actions of the events, many conversations
// known only by their meta.json, and damaged entries.
func openhandsStateCases() []growthCase {
	conversation := openhandsConversation(1)
	return []growthCase{
		{kind: "base_state.json", shape: shapeLargeShared + " (blocked actions naming one action)",
			files: func(t *testing.T, n int) []growthFile {
				blocked := joined(n, func(i int) string { return `"a":"` + numbered("hook ", i) + `"` })
				return []growthFile{text(conversation+"base_state.json", `{"agent":{"llm":{"model":"m"}},`+
					`"confirmation_policy":{"kind":"AlwaysConfirm"},"blocked_actions":{`+blocked+`}}`),
					text(openhandsEvent(conversation, 0), openhandsAction("a", long("tool")))}
			}},
		{kind: "base_state.json", shape: shapeJoins + " (every blocked action has an event)", n: 16,
			files: func(t *testing.T, n int) []growthFile {
				blocked := joined(n, func(i int) string { return `"` + numbered("a", i) + `":"hook"` })
				return append(openhandsEvents(conversation, n, func(i int) string {
					return openhandsAction(numbered("a", i), "bash")
				}), text(conversation+"base_state.json", `{"agent":{"llm":{"model":"m"}},"blocked_actions":{`+blocked+`}}`))
			}},
		{kind: "base_state.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(conversation+"base_state.json", []byte(`{"agent":{"llm":{"model":"m"}},"stats":`+
				`{"usage_to_metrics":{`+joined(n, func(i int) string { return `"` + numbered("u", i) + `":7` })+
				`}},"confirmation_policy":{"kind":"x","confirm_unknown":`+nestedArrays(n, "1")+`}}`))
		}},
		{kind: "meta.json", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return openhandsConversation(i) + "meta.json" },
				func(i int) string { return `{"title":` })
		}},
		{kind: "meta.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(conversation+"meta.json", []byte(nestedObjects(n, "title", `"x"`)+`x`))
		}},
	}
}

// openhandsHistoryCases returns the cases of the prompt histories, which
// look their project up among the conversations, and of the legacy
// sessions, which list their events directory.
func openhandsHistoryCases() []growthCase {
	return []growthCase{
		{kind: "prompt_history.json", shape: shapeCarried + " (the project's working directory)",
			files: func(t *testing.T, n int) []growthFile {
				directory := "/" + long("w")
				return []growthFile{openhandsState(openhandsConversation(1), directory),
					{rel: openhandsHistory(directory), data: []byte("[" + joined(n, func(i int) string {
						return `{"text":"` + numbered("prompt ", i) + `","timestamp":"2026-01-01T00:00:00Z"}`
					}) + "]")}}
			}},
		{kind: "prompt_history.json", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			var files []growthFile
			for k := range n {
				directory := numbered("/w", k)
				files = append(files, openhandsState(openhandsConversation(k), directory),
					text(openhandsHistory(directory), `[{"text":"p","timestamp":"2026-01-01T00:00:00Z"},7]`))
			}
			return files
		}},
		{kind: "legacy event", shape: shapeManyFiles + " (one session's events)", n: 32,
			files: func(t *testing.T, n int) []growthFile {
				return manyFiles(n, func(i int) string { return ".openhands/sessions/big/events/" + strconv.Itoa(i) + ".json" },
					func(i int) string { return `{"timestamp":"2026-01-01T00:00:00"}` })
			}},
		{kind: "legacy event", shape: shapeManyFiles + " (many sessions)", n: 16,
			files: func(t *testing.T, n int) []growthFile {
				return manyFiles(n, func(i int) string { return ".openhands/sessions/" + numbered("s", i) + "/events/0.json" },
					func(i int) string {
						if i%2 == 1 {
							return `{"timestamp":`
						}
						return `{"timestamp":"2026-01-01T00:00:00"}`
					})
			}},
	}
}
