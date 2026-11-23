package codex

import (
	"maps"
	"math"
	"math/big"
	"math/bits"
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// referenceNear is the pairing test read directly: two integer ts values
// at most 2 apart, their difference taken without a size limit, else the
// two as floats at most 2 apart.
func referenceNear(first, second sqlitefile.Value) bool {
	if first.Type == sqlitefile.Integer && second.Type == sqlitefile.Integer {
		var difference big.Int
		difference.Sub(big.NewInt(first.Int), big.NewInt(second.Int))
		return difference.CmpAbs(big.NewInt(2)) <= 0
	}
	left, leftOK := numberOf(first)
	right, rightOK := numberOf(second)
	return leftOK && rightOK && left-right <= 2 && right-left <= 2
}

// referencePairs pairs each request with the first unpaired exec near it
// by reading every exec row, and returns the exec position each request
// took or -1, with the number of exec rows read.
func referencePairs(execs []logRecord, requests []sqlitefile.Value) ([]int, int) {
	paired := map[int]bool{}
	taken, read := make([]int, len(requests)), 0
	for i, request := range requests {
		taken[i] = -1
		for position, exec := range execs {
			read++
			if !paired[position] && referenceNear(exec.timestamp, request) {
				paired[position], taken[i] = true, position
				break
			}
		}
	}
	return taken, read
}

// randomStamp returns a ts value: mostly integers and floats close
// together, sometimes text, NULL, an infinite float or an extreme integer.
func randomStamp(source *rand.Rand) sqlitefile.Value {
	switch roll := source.IntN(20); {
	case roll < 9:
		return sqlitefile.IntValue(int64(source.IntN(12)))
	case roll < 15:
		return sqlitefile.FloatValue(float64(source.IntN(48)) / 4)
	case roll < 16:
		return sqlitefile.TextValue(strconv.Itoa(source.IntN(12)))
	case roll < 17:
		return sqlitefile.Value{}
	case roll < 18:
		return sqlitefile.FloatValue(math.Inf(1 - 2*source.IntN(2)))
	case roll < 19:
		return sqlitefile.IntValue(math.MaxInt64 - int64(source.IntN(4)))
	}
	return sqlitefile.IntValue(math.MinInt64 + int64(source.IntN(4)))
}

func TestPairingTakesTheFirstUnpairedExecRowNearEachRequest(t *testing.T) {
	for seed := range uint64(400) {
		t.Run("rows from seed "+strconv.FormatUint(seed, 10), func(t *testing.T) {
			source := rand.New(rand.NewPCG(seed, 527))
			execs := make([]logRecord, source.IntN(30))
			for i := range execs {
				execs[i] = logRecord{rowid: i + 1, timestamp: randomStamp(source)}
			}
			requests := make([]sqlitefile.Value, source.IntN(30))
			for i := range requests {
				requests[i] = randomStamp(source)
			}
			want, _ := referencePairs(execs, requests)
			pairs := newExecPairs(execs)
			for i, request := range requests {
				if got := pairs.take(request); got != want[i] {
					t.Fatalf("request %d at %+v took %d, want %d", i, request, got, want[i])
				}
			}
		})
	}
}

func TestNearIntegersDoNotOverflow(t *testing.T) {
	tests := []struct {
		name        string
		left, right int64
		want        bool
	}{
		{"the two ends of the range", math.MinInt64, math.MaxInt64, false},
		{"the least integer and zero", math.MinInt64, 0, false},
		{"zero and the least integer", 0, math.MinInt64, false},
		{"two apart at the top", math.MaxInt64, math.MaxInt64 - 2, true},
		{"three apart at the bottom", math.MinInt64, math.MinInt64 + 3, false},
		{"equal", 7, 7, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := intsWithin(test.left, test.right, pairGap); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
			execs := []logRecord{{rowid: 1, timestamp: sqlitefile.IntValue(test.left)}}
			if got := newExecPairs(execs).take(sqlitefile.IntValue(test.right)) == 0; got != test.want {
				t.Errorf("paired %v, want %v", got, test.want)
			}
		})
	}
}

func TestARequestFarFromEveryExecRowIsItsOwnDecision(t *testing.T) {
	approval := func(rowid int, stamp int64, target, body string) logRecord {
		kind := map[string]string{handlersTarget: "exec", outgoingTarget: "request"}[target]
		return logRecord{rowid: rowid, timestamp: sqlitefile.IntValue(stamp), target: target, body: body, kind: kind}
	}
	records := []logRecord{
		approval(1, math.MinInt64, handlersTarget, `op=ExecApproval { id: "call", decision: approved`),
		approval(2, 0, outgoingTarget, `CommandExecutionRequestApproval { request_id: "7", decision: approved`),
	}
	var problems parserkit.Problems
	got := decisions(records, &problems)
	if _, ok := got[2]; !ok || len(got) != 2 {
		t.Errorf("decisions %v: the request at ts 0 is not paired with the exec row at the least ts",
			slices.Collect(maps.Keys(got)))
	}
}

func TestPairingCostsALogarithmPerRequest(t *testing.T) {
	const size = 2000
	execs := make([]logRecord, size)
	for i := range size {
		execs[i] = logRecord{rowid: i + 1, timestamp: sqlitefile.IntValue(int64(10 * i))}
	}
	pairs := newExecPairs(execs)
	pairs.work = parserkit.CountingWork()
	for i := range size {
		pairs.take(sqlitefile.IntValue(int64(10*(size-i) + 1000000)))
	}
	// Per request and group: two binary searches of at most bits.Len(size)
	// comparisons each, and a tree query reading at most two cells a level.
	depth := bits.Len(size)
	steps, limit := pairs.work.Count("pairing steps"), size*2*(2*depth+2*(depth+1))
	if steps == 0 || steps > limit {
		t.Errorf("%d steps for %d requests over %d exec rows, over the bound %d", steps, size, size,
			limit)
	}
}
