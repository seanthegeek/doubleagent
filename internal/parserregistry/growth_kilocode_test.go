package parserregistry

import (
	"strconv"
	"strings"
	"testing"
)

// The Kilo Code growth cases: the pre-fork extension's tasks, Kilo's own
// share markers, logs, retention state, memory digests, config and
// sandbox files, and the OpenCode stores under Kilo's directories.

// The directories of a case.
const (
	kilocodeTasks  = ".config/Code/User/globalStorage/kilocode.kilo-code/tasks/"
	kilocodeData   = ".local/share/kilo/"
	kilocodeTaskID = "8f1c2b7e-1"
	kilocodeMemory = kilocodeData + "memory/proj-0123456789ab/"
)

// kilocodeMessage returns a pre-fork task's chat message of a role holding
// one block, given as JSON.
func kilocodeMessage(role, block string) string {
	return `{"role":"` + role + `","ts":1790935201000,"content":[` + block + `]}`
}

// kilocodeLogLine returns a log line at a second of the hour with the
// fields and message given.
func kilocodeLogLine(i int, rest string) string {
	return "INFO  2026-10-03T10:" + strconv.Itoa(10+i/60%50) + ":" + strconv.Itoa(10+i%50) +
		" +12ms service=kilo-sessions " + rest
}

// kilocodeGrowth returns the Kilo Code growth set.
func kilocodeGrowth() growthSet {
	cases := opencodeCases(opencodeLayout{data: ".local/share/kilo", state: ".local/state/kilo", database: "kilo.db"})
	for _, group := range [][]growthCase{kilocodeTaskCases(), kilocodeFileCases(), kilocodeDigestCases(),
		kilocodeSettingCases()} {
		cases = append(cases, group...)
	}
	return growthSet{parser: "kilo-code", cases: cases}
}

// kilocodeTaskCases returns the cases of the pre-fork extension's tasks.
func kilocodeTaskCases() []growthCase {
	const kind = "pre-fork task"
	rel := kilocodeTasks + kilocodeTaskID + "/api_conversation_history.json"
	text := func(i int) string { return kilocodeMessage("user", `{"type":"text","text":"hi"}`) }
	return []growthCase{
		{kind: kind, shape: shapeCarried + " (the history item's workspace)", files: func(t *testing.T, n int) []growthFile {
			item := growthFile{rel: kilocodeTasks + kilocodeTaskID + "/history_item.json",
				data: []byte(`{"id":"` + kilocodeTaskID + `","ts":1790935200000,"task":"fix tests","workspace":"/` +
					long("p") + `"}`)}
			return []growthFile{item, {rel: rel, data: []byte("[" + joined(n, text) + "]")}}
		}},
		{kind: kind, shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			index := growthFile{rel: kilocodeTasks + "_index.json", data: []byte(`{"entries":[` + joined(n, func(i int) string {
				return `{"id":"` + numbered("t", i) + `","ts":1790935200000,"task":"fix tests","workspace":"/srv/proj"}`
			}) + `]}`)}
			return append(manyFiles(n, func(i int) string {
				return kilocodeTasks + numbered("t", i) + "/api_conversation_history.json"
			}, func(i int) string { return "[" + text(i) + "]" }), index)
		}},
		{kind: kind, shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(rel, []byte("["+joined(n, func(i int) string {
				return kilocodeMessage("assistant", `{"type":"tool_use","id":"toolu_1","name":"read_file",`+
					`"input":{"path":"a.py"}}`)
			})+"]"))
		}},
		{kind: kind, shape: shapeSharedID + " in the index", files: func(t *testing.T, n int) []growthFile {
			index := growthFile{rel: kilocodeTasks + "_index.json", data: []byte(`{"entries":[` + joined(n, func(i int) string {
				return `{"id":"` + kilocodeTaskID + `","ts":` + strconv.Itoa(1790935200000+i) + `,"task":"fix tests",` +
					`"workspace":"/w` + strconv.Itoa(i) + `"}`
			}) + `]}`)}
			return []growthFile{index, {rel: rel, data: []byte("[" + joined(n, func(i int) string {
				return `{"role":"user","content":"hi"}`
			}) + "]")}}
		}},
		{kind: kind, shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(rel, []byte("["+joined(n, func(i int) string { return kilocodeMessage("user", "1") })+","+
				kilocodeMessage("user", `{"type":"tool_result","tool_use_id":"t","content":`+nestedArrays(n, `"x"`)+`}`)+
				"]"))
		}},
	}
}

// kilocodeFileCases returns the cases of the share markers, the logs and
// the retention state.
func kilocodeFileCases() []growthCase {
	return []growthCase{
		{kind: "session share", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return kilocodeData + "storage/session_share/" + numbered("ses_", i) + ".json" },
				func(i int) string {
					return `{"id":"ks_01JAX","ingestPath":"/api/session/ks_01JAX/ingest","url":"https://app.kilo.ai/s/x"}`
				})
		}},
		{kind: "session share", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(kilocodeData+"storage/session_share/ses_1.json", []byte(`{"id":`+nestedArrays(n, `"k"`)+
				`,"url":`+nestedObjects(n, "url", "1")+`}`))
		}},
		{kind: "log", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(kilocodeData+"log/2026-10-03T100000.log", lines(n, func(i int) string {
				return kilocodeLogLine(i, "sessionID=ses_01JAX startedAt=2026-10-03T10:00:09.250Z "+
					"model=anthropic/claude-sonnet-4-5 directory=/srv/proj remote session started")
			}))
		}},
		{kind: "log", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(kilocodeData+"log/2026-10-03T100000.log", lines(n, func(i int) string {
				if i%2 == 0 {
					return kilocodeLogLine(i, "remote session ended")
				}
				return kilocodeLogLine(i, "connected=true remote connection enabled")
			}))
		}},
		{kind: "retention state.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(kilocodeData+"retention/state.json", []byte(`{"at":1791028800000,"deleted":`+
				nestedArrays(n, "1")+`,"scanned":`+nestedObjects(n, "a", "1")+`,"failed":[`+
				joined(n, func(i int) string { return "{}" })+`]}`))
		}},
	}
}

// kilocodeDigestCases returns the cases of the memory digests.
func kilocodeDigestCases() []growthCase {
	digest := func(i int) string {
		return "# Session " + numbered("ses_", i) + "\nUpdated: 2026-10-03T11:00:00.000Z\nTopic: tests\n\n## Summary\n" +
			"first line\nsecond line\n"
	}
	digestRel := func(i int) string { return kilocodeMemory + "sessions/" + numbered("d", i) + ".md" }
	return []growthCase{
		{kind: "memory digest", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			manifest := text(kilocodeMemory+"manifest.json", `{"canonical":"/srv/proj","pad":"`+strings.Repeat("x", 100*n)+`"}`)
			return append(manyFiles(n, digestRel, digest), manifest)
		}},
		{kind: "memory digest", shape: shapeCarried + " (the manifest's canonical root)", files: func(t *testing.T, n int) []growthFile {
			return append(manyFiles(n, digestRel, digest), text(kilocodeMemory+"manifest.json",
				`{"canonical":"/`+long("p")+`"}`))
		}},
		{kind: "memory digest", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(digestRel(0), lines(n, func(i int) string { return "# Session " + strings.Repeat("#", i%5) }))
		}},
	}
}

// kilocodeSettingCases returns the cases of the config and sandbox files.
func kilocodeSettingCases() []growthCase {
	const policies, preferences = ".local/state/kilo-sandbox-policy/", ".local/state/kilo-sandbox-preference/"
	return []growthCase{
		{kind: "config", shape: shapeCarried + " (a permission's key)", files: func(t *testing.T, n int) []growthFile {
			return one(".config/kilo/kilo.jsonc", []byte("// rules\n{\"permission\":{\""+long("bash")+"\":{"+
				joined(n, func(i int) string { return `"` + numbered("git ", i) + `":"allow"` })+"}}}"))
		}},
		{kind: "config", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".config/kilo/kilo.json", []byte(`{"permission":{`+joined(n, func(i int) string {
				return `"` + numbered("p", i) + `":"ask"`
			})+`,"deep":{"x":`+nestedArrays(n, `"allow"`)+`}}}`))
		}},
		{kind: "sandbox policy", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return policies + numbered("a", i) + "/b.json" }, func(i int) string {
				return `{"enabled":true,"mode":"proxy","allowedHosts":["registry.npmjs.org"],"version":3}`
			})
		}},
		{kind: "sandbox policy", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(policies+"a/b.json", []byte(`{`+joined(n, func(i int) string {
				return `"` + numbered("k", i) + `":` + strconv.Itoa(i)
			})+`,"writablePaths":`+nestedArrays(n, `"/srv/proj"`)+`}`))
		}},
		{kind: "sandbox preference", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return preferences + numbered("b", i) + ".json" }, func(i int) string {
				if i%2 == 0 {
					return "false"
				}
				return nestedArrays(i%8, "true")
			})
		}},
	}
}
