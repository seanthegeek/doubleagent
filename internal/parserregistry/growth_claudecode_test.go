package parserregistry

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The Claude Code growth cases: the history and its paste cache, the
// session transcripts and their persisted tool outputs, and the standing
// files.

const claudeSession = "11111111-2222-3333-4444-555555555555"

// claudeLine returns a transcript line of the session: a user record of
// the given uuid holding content.
func claudeLine(uuid, content string) string {
	return `{"type":"user","uuid":"` + uuid + `","sessionId":"` + claudeSession + `","cwd":"/p",` +
		`"timestamp":"2026-01-02T03:04:05.000Z","message":{"role":"user","content":` + content + `}}`
}

// claudeToolResult returns a tool_result block whose content is the stub
// of an output saved to tool-results/name.
func claudeToolResult(i int, name string) string {
	return `[{"type":"tool_result","tool_use_id":"t` + strconv.Itoa(i) + `","content":"<persisted-output>\nOutput ` +
		`too large. Full output saved to: /x/` + claudeSession + `/tool-results/` + name + `\n</persisted-output>"}]`
}

// spelling returns name with its ASCII letters upper-cased where the bits
// of i say so, so each i below 2^letters spells the name differently.
func spelling(name string, i int) string {
	out := []byte(name)
	bit := 0
	for k, character := range out {
		if 'a' <= character && character <= 'z' {
			if i>>bit&1 == 1 {
				out[k] = character - 'a' + 'A'
			}
			bit++
		}
	}
	return string(out)
}

// claudecodeGrowth returns the Claude Code growth set.
func claudecodeGrowth() growthSet {
	return growthSet{parser: "claude-code", cases: slices.Concat(claudecodeHistory(), claudecodeTranscripts(),
		claudecodeStanding())}
}

// claudecode paths of the cases: the project directory, the session's
// transcript and its persisted tool outputs.
const (
	claudeProject     = ".claude/projects/-p/"
	claudeTranscript  = claudeProject + claudeSession + ".jsonl"
	claudeToolResults = claudeProject + claudeSession + "/tool-results/"
)

// claudecodeHistory returns the cases of history.jsonl.
func claudecodeHistory() []growthCase {
	return []growthCase{
		{kind: "history.jsonl", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{text(".claude/paste-cache/h1.txt", long("paste")),
				{rel: ".claude/history.jsonl", data: lines(n, func(i int) string {
					return `{"display":"[Pasted text #1]","pastedContents":{"1":{"contentHash":"h1"}},` +
						`"timestamp":` + strconv.Itoa(1767000000000+i) + `,"project":"/p","sessionId":"s"}`
				})}}
		}},
		{kind: "history.jsonl", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			first := `{"display":"a","timestamp":1767000000000,"project":"/` + long("p") + `","sessionId":"s"}`
			return one(".claude/history.jsonl", append([]byte(first+"\n"), lines(n, func(i int) string {
				return `{"display":"b","timestamp":` + strconv.Itoa(1767000000001+i) + `,"sessionId":"s"}`
			})...))
		}},
		{kind: "history.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".claude/history.jsonl", append(lines(n, func(i int) string { return `{"display": [` }),
				[]byte(`{"display":`+nestedArrays(n, `"x"`)+`}`+"\n")...))
		}},
	}
}

// claudecodeTranscripts returns the cases of the session transcripts.
func claudecodeTranscripts() []growthCase {
	return []growthCase{
		{kind: "session transcript", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{text(claudeToolResults+"abcdefghij.txt", long("output")),
				{rel: claudeTranscript, data: lines(n, func(i int) string {
					return claudeLine(numbered("u", i), claudeToolResult(i, "abcdefghij.txt"))
				})}}
		}},
		{kind: "session transcript", shape: shapeLargeShared + " by many spellings", n: 32,
			files: func(t *testing.T, n int) []growthFile {
				return []growthFile{text(claudeToolResults+"abcdefghij.txt", strings.Repeat("o", 100*n)),
					{rel: claudeTranscript, data: lines(n, func(i int) string {
						return claudeLine(numbered("u", i), claudeToolResult(i, spelling("abcdefghij.txt", i)))
					})}}
			}},
		{kind: "session transcript", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(claudeTranscript, lines(n, func(i int) string { return claudeLine("same", `"hello"`) }))
		}},
		{kind: "session transcript", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			first := `{"type":"summary","summary":"` + long("s") + `","leafUuid":"u0"}`
			return one(claudeTranscript, append([]byte(first+"\n"), lines(n, func(i int) string {
				return claudeLine(numbered("u", i), `"hi"`)
			})...))
		}},
		{kind: "session transcript", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(claudeTranscript, append(lines(n, func(i int) string { return `{"type":"user","message":` }),
				[]byte(claudeLine("deep", nestedArrays(n, `"x"`))+"\n")...))
		}},
		{kind: "session transcript", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{text(claudeTranscript, claudeLine("u", claudeToolResult(0, "a.txt"))+"\n"),
				text(claudeToolResults+"a.txt", "output")}
			return append(files, manyFiles(n, func(i int) string {
				return claudeProject + claudeSession + "/subagents/agent-a" + strconv.Itoa(i) + ".jsonl"
			}, func(i int) string { return claudeLine(numbered("s", i), claudeToolResult(i, "a.txt")) + "\n" })...)
		}},
	}
}

// claudecodeStanding returns the cases of the registry, settings, global
// config and the remote files.
func claudecodeStanding() []growthCase {
	return []growthCase{
		{kind: "session registry", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return ".claude/sessions/" + strconv.Itoa(1000+i) + ".json" },
				func(i int) string {
					return `{"pid":` + strconv.Itoa(1000+i) + `,"sessionId":"` + claudeSession + `","cwd":"/p",` +
						`"startedAt":1767000000000,"kind":"interactive","bridgeSessionId":"b"}`
				})
		}},
		{kind: "session registry", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".claude/sessions/1.json", []byte(`{"pid":1,"sessionId":"`+long("s")+`","cwd":"/p",`+
				`"bridgeSessionId":"`+long("b")+`"}`))
		}},
		{kind: "settings.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".claude/settings.json", []byte(`{"permissions":{"allow":[`+joined(n, func(i int) string {
				return `"Bash(` + numbered("c", i) + `)"`
			})+`],"defaultMode":"`+long("mode")+`"},"model":"`+long("model")+`"}`))
		}},
		{kind: "settings.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".claude/settings.json", []byte(`{"permissions":{"allow":`+nestedArrays(n, `"x"`)+
				`,"deny":[`+joined(n, func(i int) string { return `{"bad":` + strconv.Itoa(i) + `}` })+`]}}`))
		}},
		{kind: ".claude.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".claude.json", []byte(`{"projects":{"/`+long("p")+`":{"allowedTools":[`+
				joined(n, func(i int) string { return `"` + numbered("t", i) + `"` })+`]}}}`))
		}},
		{kind: ".claude.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".claude.json", []byte(`{"projects":{`+joined(n, func(i int) string {
				return `"/p` + strconv.Itoa(i) + `":` + strconv.Itoa(i)
			})+`,"/deep":{"allowedTools":`+nestedArrays(n, "1")+`}}}`))
		}},
		{kind: "ccr-tip.json", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return claudeProject + numbered("s", i) + "/ccr-tip.json" },
				func(i int) string { return `{"eventId":"e` + strconv.Itoa(i) + `","updatedAt":"2026-01-01"}` })
		}},
		{kind: "bridge-pointer.json", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return ".claude/projects/" + numbered("p", i) + "/bridge-pointer.json" },
				func(i int) string { return `{"environmentId":"e","sessionId":"s","source":"cli"}` })
		}},
	}
}
