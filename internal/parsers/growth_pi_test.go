package parsers

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"strconv"
	"testing"
)

// The pi growth cases: the session files and the parent files their forks
// look up, the trust, settings, crash and debug files, the experimental
// session metadata and the three exports.

// piSessions is the directory of one project's sessions.
const piSessions = ".pi/agent/sessions/--p--/"

// piHeader returns a session header of the given id in cwd, a fork of
// parent when parent is not empty.
func piHeader(identifier, cwd, parent string) string {
	fork := ""
	if parent != "" {
		fork = `,"parentSession":"` + parent + `"`
	}
	return `{"type":"session","version":3,"id":"` + identifier + `","timestamp":"2026-01-01T00:00:00Z","cwd":"` + cwd +
		`"` + fork + `}`
}

// piMessage returns a user message entry of the given id and text.
func piMessage(identifier, content string) string {
	return `{"type":"message","id":"` + identifier + `","parentId":null,"timestamp":"2026-01-01T00:00:01Z",` +
		`"message":{"role":"user","content":"` + content + `"}}`
}

// piSession returns a session file's lines: the header, then n message
// entries.
func piSession(header string, n int) []byte {
	return append([]byte(header+"\n"), lines(n, func(i int) string {
		return piMessage(numbered("e", i), numbered("entry ", i))
	})...)
}

// piGreek is a parent session's name in Greek letters, which a disk that
// folds case matches in any case as it does ASCII letters.
const piGreek = "αβγδεζηθικλμνξοπρστυφχψω"

// piGreekSpelling returns piGreek with its letters in capitals where the
// bits of i say so: each Greek capital is its small letter less 0x20.
func piGreekSpelling(i int) string {
	letters := []rune(piGreek)
	for k := range letters {
		if i>>k&1 == 1 {
			letters[k] -= 0x20
		}
	}
	return string(letters)
}

// piZip returns a zip archive holding the members, each a name and its
// content, in order, stored without compression so that the archive's
// size is its members' size.
func piZip(t *testing.T, members [][2]string) []byte {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, member := range members {
		file, err := writer.CreateHeader(&zip.FileHeader{Name: member[0], Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(member[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

// piGrowth returns the pi growth set.
func piGrowth() growthSet {
	var cases []growthCase
	for _, part := range [][]growthCase{piSessionCases(), piFileCases(), piCrashCases(), piExportCases()} {
		cases = append(cases, part...)
	}
	return growthSet{parser: "pi", cases: cases}
}

// piForks returns a parent session of n entries and n forks naming it,
// fork i by name(i).
func piForks(n int, parentName string, name func(i int) string) []growthFile {
	files := []growthFile{{rel: piSessions + parentName + ".jsonl", data: piSession(piHeader("P", "/p", ""), n)}}
	return append(files, manyFiles(n, func(i int) string { return piSessions + numbered("fork", i) + ".jsonl" },
		func(i int) string {
			return piHeader(numbered("F", i), "/p", "/home/u/"+piSessions+name(i)+".jsonl") + "\n" +
				piMessage("e0", "entry 0") + "\n" + piMessage(numbered("own", i), "mine") + "\n"
		})...)
}

// piSessionCases returns the cases of the session files: forks naming one
// parent, by one name and by many case spellings, a cwd every row
// inherits, entries sharing one id, and damaged lines.
func piSessionCases() []growthCase {
	return []growthCase{
		{kind: "session", shape: shapeLargeShared + " (forks of one parent)", n: 32,
			files: func(t *testing.T, n int) []growthFile {
				return piForks(n, "parent", func(i int) string { return "parent" })
			}},
		{kind: "session", shape: shapeLargeShared + " (forks of one parent by Greek case spellings)", n: 32,
			foldsCase: true, known: "on a file system that folds case, allocations grow 3.5 times from n=32 to " +
				"64: each Greek case spelling of the parent's name, which the entry key folds in ASCII only, " +
				"reads the parent again (pi/pi.go parentKeys, parser/entrykey.go KeyOf)",
			files: func(t *testing.T, n int) []growthFile {
				return piForks(n, piGreek, func(i int) string { return piGreekSpelling(i + 1) })
			}},
		{kind: "session", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(piSessions+"a_s1.jsonl", piSession(piHeader("s1", "/"+long("p"), ""), n))
		}},
		{kind: "session", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(piSessions+"a_s1.jsonl", append([]byte(piHeader("s1", "/p", "")+"\n"),
				lines(n, func(i int) string { return piMessage("same", "hello") })...))
		}},
		{kind: "session", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(piSessions+"a_s1.jsonl", append([]byte(piHeader("s1", "/p", "")+"\n"),
				lines(n, func(i int) string {
					if i == n-1 {
						return `{"type":"message","id":"deep","message":{"role":"user","content":` +
							nestedArrays(n, `"x"`) + `}}`
					}
					return `{"type":"message",`
				})...))
		}},
	}
}

// piCrash returns a crashes.json record of a session file.
func piCrash(i int, sessionFile string) string {
	return `{"kind":"uncaughtException","message":"boom","version":"0.83.0","sessionFile":"` + sessionFile +
		`","timestamp":` + strconv.Itoa(1767000000000+i) + `,"cwd":"/p","stack":"Error: boom"}`
}

// piFileCases returns the cases of the trust, settings and debug files.
func piFileCases() []growthCase {
	return []growthCase{
		{kind: "trust.json", shape: shapeSharedID, n: 4096, files: func(t *testing.T, n int) []growthFile {
			return one(".pi/agent/trust.json", []byte("{"+joined(n, func(i int) string { return `"/p":true` })+"}"))
		}},
		{kind: "trust.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".pi/agent/trust.json", []byte("{"+joined(n, func(i int) string {
				return `"` + numbered("/p", i) + `":{"bad":` + strconv.Itoa(i) + `}`
			})+`,"/deep":`+nestedArrays(n, "true")+"}"))
		}},
		{kind: "settings.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(".pi/agent/settings.json", []byte(`{"defaultProjectTrust":`+nestedObjects(n, "a",
				"["+joined(n, func(i int) string { return `{"bad":1}` })+"]")+`,"theme":"dark"}`))
		}},
		{kind: "pi-debug.log", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".pi/agent/pi-debug.log", append([]byte("Debug output at 2026-01-01T00:00:00Z\nscreen\n"+
				"=== Agent messages (JSONL) ===\n"), lines(n, func(i int) string {
				return `{"role":"user","content":"the same","timestamp":1767000000000}`
			})...))
		}},
		{kind: "pi-debug.log", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".pi/agent/pi-debug.log", append([]byte("Debug output at 2026-01-01T00:00:00Z\n"+
				"=== Agent messages (JSONL) ===\n"), lines(n, func(i int) string {
				if i == n-1 {
					return `{"role":"user","content":` + nestedArrays(n, `"x"`) + `}`
				}
				return `{"role":`
			})...))
		}},
	}
}

// piCrashCases returns the cases of the crash records, the crash logs and
// the experimental session metadata.
func piCrashCases() []growthCase {
	return []growthCase{
		{kind: "crashes.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".pi/agent/crashes.json", []byte("["+joined(n, func(i int) string {
				return piCrash(i, "/home/u/"+piSessions+"a_same.jsonl")
			})+"]"))
		}},
		{kind: "crashes.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".pi/agent/crashes.json", []byte("["+joined(n, strconv.Itoa)+`,{"stack":`+
				nestedArrays(n, `"x"`)+"}]"))
		}},
		{kind: "crash log", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".pi/agent/pi-tui-crash.log", lines(n, func(i int) string {
				return "Crash at 2026-01-01T00:00:00Z\nTerminal width: 80\n=== All rendered lines ===\nline " +
					strconv.Itoa(i)
			}))
		}},
		{kind: "crash log", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".pi/agent/pi-crash.log", lines(n, func(i int) string { return numbered("stray ", i) }))
		}},
		{kind: "session meta.json", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string {
				return ".pi/agent/experimental/sessions/" + numbered("s", i) + "/meta.json"
			},
				func(i int) string { return `{"cwd":"/p","createdAt":"2026-01-01T00:00:00Z"}` })
		}},
		{kind: "session meta.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(".pi/agent/experimental/sessions/s1/meta.json", []byte(`{"cwd":`+nestedArrays(n, `"x"`)+
				`,"createdAt":[`+joined(n, strconv.Itoa)+`]}`))
		}},
	}
}

// piHTML returns an HTML export whose session-data element holds the
// session of the header and entries as base64 JSON.
func piHTML(header, entries string) []byte {
	data := `{"header":` + header + `,"entries":[` + entries + `],"leafId":"e0"}`
	return []byte(`<html><body><script id="session-data" type="application/json">` +
		base64.StdEncoding.EncodeToString([]byte(data)) + "</script></body></html>")
}

// piExportCases returns the cases of the HTML and JSONL exports and the
// bug report zip: a cwd every copied row inherits, entries sharing one id,
// members given many times, and damaged entries.
func piExportCases() []growthCase {
	entries := func(n int) string {
		return joined(n, func(i int) string { return piMessage(numbered("e", i), "hi") })
	}
	return []growthCase{
		{kind: "HTML export", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one("pi-session-a.html", piHTML(piHeader("s1", "/"+long("p"), ""), entries(n)))
		}},
		{kind: "HTML export", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one("pi-session-a.html", piHTML(piHeader("s1", "/p", ""), joined(n, strconv.Itoa)+
				`,{"type":"message","message":{"role":"user","content":`+nestedArrays(n, `"x"`)+`}}`))
		}},
		{kind: "JSONL export", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one("session-2026-01-01.jsonl", piSession(piHeader("s1", "/"+long("p"), ""), n))
		}},
		{kind: "JSONL export", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one("session-2026-01-01.jsonl", append([]byte(piHeader("s1", "/p", "")+"\n"),
				lines(n, func(i int) string { return piMessage("same", "hi") })...))
		}},
		{kind: "bug report zip", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one("pi-bug-report-a.zip", piZip(t, [][2]string{
				{"report.json", `{"id":"r1","session":{"id":"s1","cwd":"/p"}}`},
				{"diagnostics.json", `{"crashes":[` + joined(n, func(i int) string { return piCrash(i, "x_s1.jsonl") }) + `]}`},
				{"session.jsonl", string(piSession(piHeader("s1", "/"+long("p"), ""), n))}}))
		}},
		{kind: "bug report zip", shape: shapeSharedID + " (one member name many times)",
			files: func(t *testing.T, n int) []growthFile {
				members := [][2]string{{"report.json", `{"id":"r1"}`}, {"diagnostics.json", `{"assistant":[]}`}}
				for i := range n {
					members = append(members, [2]string{"session.jsonl", string(piSession(piHeader(numbered("s", i), "/p", ""), 2))})
				}
				return one("pi-bug-report-a.zip", piZip(t, members))
			}},
		{kind: "bug report zip", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one("pi-bug-report-a.zip", piZip(t, [][2]string{{"report.json", `{"id":` + nestedArrays(n, "1") + `}`},
				{"diagnostics.json", `{"assistant":[` + joined(n, strconv.Itoa) + `],"crashes":7}`}}))
		}},
	}
}
