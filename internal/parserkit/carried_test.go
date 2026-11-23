package parserkit

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// carriedRun is one run of produce, then of later, each as one parse of an
// artifact of one home with setting, as an analysis run makes them: it
// returns the rows and sessions as the output writes them, each string
// collected first, and the rows as the parses gave them.
func carriedRun(setting secrets.Setting, produce, later func(artifact *input.Artifact, emit Emit)) (
	[]timeline.Row, []timeline.SessionSummary, []timeline.Row) {
	recorder := secrets.NewRecorder(setting)
	home := &input.Home{Host: "host", User: "user", Secrets: recorder}
	var raw []timeline.Row
	for i, each := range []func(artifact *input.Artifact, emit Emit){produce, later} {
		if each == nil {
			continue
		}
		artifact := &input.Artifact{Original: "/home/user/file" + strconv.Itoa(i), Home: home}
		_, _ = Within(artifact, func(emit Emit) error { each(artifact, emit); return nil },
			func(row timeline.Row) { raw = append(raw, row) })
	}
	sessions := timeline.Summarise(slices.Clone(raw), timeline.MarkAgents{})
	for _, row := range raw {
		row.Collect(recorder)
	}
	for _, session := range sessions {
		session.Collect(recorder)
	}
	rows := make([]timeline.Row, len(raw))
	for i, row := range raw {
		rows[i] = row.Withheld(recorder)
	}
	for i, session := range sessions {
		sessions[i] = session.Withheld(recorder)
	}
	return rows, sessions, raw
}

// filler returns n bytes of a path's text, a / after every nine a's.
func filler(n int) string {
	return strings.Repeat("aaaaaaaaa/", n/10+1)[:n]
}

func TestEveryFieldButTheTextIsCutAtTheLimitWithOneProblemPerField(t *testing.T) {
	long := filler(10_000)
	mark := " [cut from 10000 bytes]"
	cut := long[:CarriedLimit-len(mark)] + mark
	for _, field := range (&timeline.Row{}).FieldsButText() {
		t.Run(field.Name+" is cut and the text is not", func(t *testing.T) {
			_, _, raw := carriedRun(secrets.Setting{}, func(artifact *input.Artifact, emit Emit) {
				for range 3 {
					row := timeline.Row{Text: long}
					for _, each := range row.FieldsButText() {
						if each.Name == field.Name {
							*each.Value = long
						}
					}
					emit(row)
				}
				emit(timeline.Row{SessionID: "short", Text: "short"})
			}, nil)
			want := timeline.Row{Text: long}
			for _, each := range want.FieldsButText() {
				if each.Name == field.Name {
					*each.Value = cut
				}
			}
			problem := ProblemPrefix + field.Name + " longer than 4096 bytes, cut to 4096 bytes (3 times)"
			if len(raw) != 5 || raw[3].Text != "short" || raw[3].SessionID != "short" || raw[4].Text != problem {
				t.Fatalf("rows %d, last two %q and %q, want 5, the short row and %q", len(raw), raw[3].Text,
					raw[len(raw)-1].Text, problem)
			}
			for _, row := range raw[:3] {
				row.Secrets = nil
				if row != want {
					t.Errorf("row %+v, want %+v", row, want)
				}
			}
			if len(cut) > CarriedLimit || len(*raw[4].FieldsButText()[0].Value) > CarriedLimit {
				t.Errorf("a cut value of %d bytes", len(cut))
			}
		})
	}
}

func TestAValueAtTheLimitIsKeptAndOneByteMoreIsCut(t *testing.T) {
	tests := []struct {
		name   string
		length int
		cut    bool
	}{
		{"a value of the limit is kept", CarriedLimit, false},
		{"a value one byte longer is cut", CarriedLimit + 1, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := filler(test.length)
			_, _, raw := carriedRun(secrets.Setting{}, func(artifact *input.Artifact, emit Emit) {
				emit(timeline.Row{Model: value})
			}, nil)
			mark := cutMark(test.length)
			want := value
			if test.cut {
				want = value[:CarriedLimit-len(mark)] + mark
			}
			if raw[0].Model != want || (len(raw) == 2) != test.cut {
				t.Errorf("model of %d bytes and %d rows, want %d bytes, cut %v", len(raw[0].Model), len(raw),
					len(want), test.cut)
			}
		})
	}
}

func TestACutValueKeepsWholeCharactersAndSaysItsLength(t *testing.T) {
	// "é" is two bytes, so one byte of filler puts a character across the
	// cut point.
	accented := "a" + strings.Repeat("é", 5000)
	mark := " [cut from 10001 bytes]"
	got := cutValue(nil, accented, CarriedLimit, false)
	if !utf8.ValidString(got) || !strings.HasSuffix(got, mark) || len(got) > CarriedLimit ||
		got != accented[:len(got)-len(mark)]+mark {
		t.Errorf("cut %q... of %d bytes, want whole characters and %q", got[:10], len(got), mark)
	}
}

func TestTwoLongValuesStayApartUnlessTheirLengthAndFirstBytesAgree(t *testing.T) {
	base := filler(5000)
	tests := []struct {
		name         string
		left, right  string
		sameSessions bool
	}{
		{"values of different lengths stay apart", base + "x", base + "xy", false},
		{"values that differ after the limit and agree in length become one", base + "x", base + "y", true},
		{"values that differ before the limit stay apart", "b" + base, "c" + base, false},
		{"URLs without credentials that differ before the limit stay apart",
			"https://a.example/" + strings.Repeat("x", 5000), "s3://bucket-bbbbb/" + strings.Repeat("y", 5000), false},
		{"URLs without credentials that differ after their scheme stay apart",
			"https://a.example/" + strings.Repeat("x", 5000), "https://b.example/" + strings.Repeat("x", 5000), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, sessions, _ := carriedRun(secrets.Setting{}, func(artifact *input.Artifact, emit Emit) {
				emit(timeline.Row{SessionID: test.left, TurnType: timeline.TurnUser})
				emit(timeline.Row{SessionID: test.right, TurnType: timeline.TurnUser})
			}, nil)
			if (len(sessions) == 1) != test.sameSessions {
				t.Errorf("%d sessions, want one: %v", len(sessions), test.sameSessions)
			}
		})
	}
}

func TestOnlyAValueWrittenLikeACutEqualsTheCutOfALongerOne(t *testing.T) {
	long := filler(5000)
	cut := long[:CarriedLimit-len(cutMark(5000))] + cutMark(5000)
	values := []string{filler(4095), filler(4096), filler(4097), long, cut}
	_, sessions, raw := carriedRun(secrets.Setting{}, func(artifact *input.Artifact, emit Emit) {
		for _, value := range values {
			emit(timeline.Row{SessionID: value, TurnType: timeline.TurnUser, TimestampUTC: "2026-01-01T00:00:00Z"})
		}
	}, nil)
	if len(sessions) != 4 || raw[3].SessionID != cut || raw[4].SessionID != cut {
		t.Errorf("%d sessions, want 4: the value written like a cut and the value it copies as one", len(sessions))
	}
}

func TestALongWindowsProjectPathIsCutInItsOneSpelling(t *testing.T) {
	spellings := []string{`c:/Users/a/` + filler(6000), `C:\Users\a\` + strings.ReplaceAll(filler(6000), "/", `\`)}
	var cut []string
	for _, spelling := range spellings {
		_, _, raw := carriedRun(secrets.Setting{}, func(artifact *input.Artifact, emit Emit) {
			emit(timeline.Row{ProjectPath: spelling})
		}, nil)
		cut = append(cut, raw[0].ProjectPath)
	}
	if cut[0] != cut[1] || timeline.CanonicalProjectPath(cut[0]) != cut[0] || !strings.HasPrefix(cut[0], `C:\Users\a\`) {
		t.Errorf("cut %q and %q, want one canonical spelling", cut[0][:20], cut[1][:20])
	}
}

// sharedPathAllocations returns the allocations of one parse that emits n
// rows sharing path as their project path.
func sharedPathAllocations(path string, n int) float64 {
	artifact := &input.Artifact{Original: "/home/user/file"}
	return testing.AllocsPerRun(3, func() {
		_, _ = Within(artifact, func(emit Emit) error {
			for range n {
				emit(timeline.Row{ProjectPath: path, TurnType: timeline.TurnUser, Text: "t"})
			}
			return nil
		}, func(timeline.Row) {})
	})
}

func TestALongProjectPathTheRowsShareCostsTheSameARowInEitherForm(t *testing.T) {
	windows := `C:\` + strings.ReplaceAll(filler(10_000), "/", `\`)
	unix := "/" + filler(10_000)
	// The rows past the first cost what every row does, whatever the form:
	// the same allocations a row, rounded, since the runtime may allocate a
	// few times of its own during a run.
	windowsRows := math.Round((sharedPathAllocations(windows, 1000) - sharedPathAllocations(windows, 500)) / 500)
	unixRows := math.Round((sharedPathAllocations(unix, 1000) - sharedPathAllocations(unix, 500)) / 500)
	if windowsRows != unixRows {
		t.Errorf("a row allocated %v times with a Windows path and %v with a Unix path", windowsRows, unixRows)
	}
}
