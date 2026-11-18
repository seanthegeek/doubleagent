package claudecode

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// bridgeURLByScan is the bridge URL lookup written as a scan of every
// record, which the indexed lookup must agree with.
func bridgeURLByScan(scan *prescan, bridge jsonvalue.Value, line int) string {
	start, end := 0, -1
	for _, other := range scan.bridgeLines {
		if other.named && bridge.IsString() && other.bridgeID == bridge.StringValue() {
			continue
		}
		if other.line < line {
			start = max(start, other.line)
		} else if other.line > line && (end < 0 || other.line < end) {
			end = other.line
		}
	}
	before := "-"
	for _, status := range scan.bridgeURLs {
		if status.line >= line && (end < 0 || status.line < end) {
			return status.url
		}
		if start < status.line && status.line < line {
			before = status.url
		}
	}
	return before
}

// randomBridges returns a prescan whose lines 1 to 40 each hold, at
// random, a bridge-session record naming one of a few bridges or none, a
// bridge_status record, or neither.
func randomBridges(source *rand.Rand) *prescan {
	scan := &prescan{}
	for line := 1; line <= 40; line++ {
		switch source.IntN(3) {
		case 0:
			bridgeID := []string{"", "a", "b", "c"}[source.IntN(4)]
			scan.bridgeLines = append(scan.bridgeLines, bridgeLine{line: line, bridgeID: bridgeID, named: bridgeID != ""})
		case 1:
			scan.bridgeURLs = append(scan.bridgeURLs, bridgeStatus{line: line, url: fmt.Sprintf("u%d", line)})
		}
	}
	return scan
}

func TestTheBridgeURLIsTheOneAScanOfEveryRecordFinds(t *testing.T) {
	bridges := []jsonvalue.Value{jsonvalue.NewString("a"), jsonvalue.NewString("b"), jsonvalue.NewString(""),
		jsonvalue.NewString("z"), jsonvalue.NewInt(1), jsonvalue.NewNull()}
	for seed := range uint64(300) {
		t.Run(fmt.Sprintf("seed %d", seed), func(t *testing.T) {
			source := rand.New(rand.NewPCG(seed, 527))
			scan := randomBridges(source)
			for line := 0; line <= 41; line++ {
				for _, bridge := range bridges {
					want := bridgeURLByScan(scan, bridge, line)
					if got := scan.bridgeURL(bridge, line, parser.Work{}); got != want {
						t.Fatalf("bridge %s at line %d: got %q, want %q (records %v, statuses %v)",
							bridge.JSON(), line, got, want, scan.bridgeLines, scan.bridgeURLs)
					}
				}
			}
		})
	}
}

// lookupWork parses a transcript in which each of n sessions names one
// bridge, with its bridge_status record, and queues and is delivered one
// prompt, and returns the bridge records and deliveries looked at.
func lookupWork(t *testing.T, n int) (bridges, deliveries int) {
	t.Helper()
	var log strings.Builder
	for i := range n {
		fmt.Fprintf(&log, `{"type":"system","subtype":"bridge_status","url":"https://example.invalid/%d"}`+"\n", i)
		fmt.Fprintf(&log, `{"type":"bridge-session","sessionId":"s%d","bridgeSessionId":"b%d","lastSequenceNum":1}`+"\n",
			i, i)
		fmt.Fprintf(&log, `{"type":"queue-operation","sessionId":"s%d","operation":"enqueue","content":"go on"}`+"\n", i)
		fmt.Fprintf(&log, `{"type":"user","sessionId":"s%d","message":{"role":"user","content":"go on"}}`+"\n", i)
	}
	home := t.TempDir()
	rel := ".claude/projects/-srv-proj/11111111-2222-4333-8444-555555555555.jsonl"
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(log.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	parsed := New()
	parsed.Work = parser.CountingWork()
	synced, queued := 0, 0
	err := parsed.Parse(&input.Artifact{Rel: rel, DiskPath: path, Home: &input.Home{DiskPath: home}},
		parser.Options{}, func(row timeline.Row) {
			if strings.Contains(row.Text, "remote control bridge") {
				synced++
			}
			if strings.HasPrefix(row.Text, "prompt queued: delivered") {
				queued++
			}
		})
	if err != nil || synced != n || queued != n {
		t.Fatalf("Parse gave %d sync rows and %d queue rows for %d sessions, error %v", synced, queued, n, err)
	}
	return parsed.Work.Count("bridge records looked at"), parsed.Work.Count("deliveries looked at")
}

func TestBridgeAndQueueLookupsGrowWithTheTranscriptNotItsSquare(t *testing.T) {
	smallBridges, smallDeliveries := lookupWork(t, 200)
	largeBridges, largeDeliveries := lookupWork(t, 2000)
	// Ten times the records costs about ten times the binary searches'
	// steps, each a little longer; a scan of every record per lookup
	// would cost a hundred times as much.
	if smallBridges == 0 || largeBridges > 15*smallBridges {
		t.Errorf("bridge records looked at: %d for 200 sessions, %d for 2000", smallBridges, largeBridges)
	}
	if smallDeliveries == 0 || largeDeliveries > 15*smallDeliveries {
		t.Errorf("deliveries looked at: %d for 200 sessions, %d for 2000", smallDeliveries, largeDeliveries)
	}
}
