package parserkit

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The secrets below are written in the letters Q and X alone, which no
// filler, mark or placeholder holds, so an output string holding either
// shows part of a secret.
const (
	fakeSecret     = "QXQXQXQXQXQXQXQX"
	fakeShort      = "QXQXQ"
	fakeURL        = "https://QX:QXQXQX@h/"
	fakeCredential = "QX:QXQXQX"
)

// recordAt says when a parse identifies its secret: never, before the
// row that holds it, after it, or in the run's next parse.
type recordAt int

const (
	recordNever recordAt = iota
	recordBefore
	recordAfter
	recordLater
)

// plantCase is a value of 10,000 bytes holding a secret near the cut, with
// the bytes the cut keeps and where the withheld part of them starts (-1
// when nothing kept is withheld).
type plantCase struct {
	name     string
	value    string
	kept     int
	withheld int
	secret   string
	when     recordAt
}

// plantCases returns the values the cut is tried on. The cut point of a
// value of 10,000 bytes is the limit less its mark.
func plantCases() []plantCase {
	point := CarriedLimit - len(cutMark(10_000))
	around := func(before int, planted string) string {
		return filler(point-before) + planted + filler(10_000-(point-before)-len(planted))
	}
	return []plantCase{
		{"a URL with a password across the cut in the value's one token",
			around(10, fakeURL), point - 2, -1, "", recordNever},
		{"a URL with a password across the cut after white space",
			around(11, " "+fakeURL), point - 2, -1, "", recordNever},
		{"a URL with a password across the cut in a short token",
			around(12, " "+fakeURL+" "), point - 3, -1, "", recordNever},
		{"a secret identified before the row, across the cut",
			around(5, fakeSecret), point, point - 5, fakeSecret, recordBefore},
		{"a secret identified after the row, across the cut",
			around(5, fakeSecret), point, point - 5, fakeSecret, recordAfter},
		{"a secret identified in the next parse, across the cut",
			around(5, fakeSecret), point, point - 5, fakeSecret, recordLater},
		{"a URL wholly after the kept bytes", around(-900, fakeURL), point, -1, "", recordNever},
		{"a secret wholly after the kept bytes", around(-900, fakeSecret), point, -1, fakeSecret, recordBefore},
	}
}

// plantedField is a place a long value is written: a field the kit cuts,
// or a key a parser writes into a row's text through Carried.
type plantedField struct {
	name string
	set  func(row *timeline.Row, artifact *input.Artifact, value string, carried *Carried)
	get  func(row timeline.Row) string
}

// plantedFields returns the project path, the session id and a settings
// key written before a leaf in a row's text.
func plantedFields() []plantedField {
	return []plantedField{
		{"project_path", func(row *timeline.Row, _ *input.Artifact, value string, _ *Carried) { row.ProjectPath = value },
			func(row timeline.Row) string { return row.ProjectPath }},
		{"session_id", func(row *timeline.Row, _ *input.Artifact, value string, _ *Carried) { row.SessionID = value },
			func(row timeline.Row) string { return row.SessionID }},
		{"settings key", func(row *timeline.Row, _ *input.Artifact, value string, carried *Carried) {
			row.Text = "permission setting: " + carried.Cut("settings key", value) + ".leaf=1"
		}, func(row timeline.Row) string {
			return strings.TrimSuffix(strings.TrimPrefix(row.Text, "permission setting: "), ".leaf=1")
		}},
	}
}

// plantedRun runs one parse emitting the planted row, a row whose text
// holds the secret, or the credential of a planted URL, alone, and the problem of a key
// written through Carried, identifying the secret when the case says.
func plantedRun(setting secrets.Setting, test plantCase, field plantedField) ([]timeline.Row,
	[]timeline.SessionSummary, []timeline.Row) {
	identify := func(artifact *input.Artifact) {
		if test.secret != "" {
			SecretText(artifact, test.secret)
		}
	}
	produce := func(artifact *input.Artifact, emit Emit) {
		if test.when == recordBefore {
			identify(artifact)
		}
		carried := NewCarried(artifact)
		row := timeline.Row{SessionID: "session", TurnType: timeline.TurnSystem, Text: "planted"}
		field.set(&row, artifact, test.value, carried)
		emit(row)
		alone := test.secret
		if strings.Contains(test.value, fakeURL) {
			alone = fakeCredential
		}
		emit(timeline.Row{SessionID: "session", Text: "alone: " + alone})
		if test.when == recordAfter {
			identify(artifact)
		}
		var problems Problems
		carried.Report(0, &problems)
		for _, problem := range problems {
			emit(timeline.Row{Text: ProblemPrefix + problem.Message})
		}
	}
	var later func(artifact *input.Artifact, emit Emit)
	if test.when == recordLater {
		later = func(artifact *input.Artifact, _ Emit) { identify(artifact) }
	}
	return carriedRun(setting, produce, later)
}

func TestACutKeepsNoPartOfASecretInAnyMode(t *testing.T) {
	mark := cutMark(10_000)
	for _, field := range plantedFields() {
		for _, test := range plantCases() {
			for _, placeholder := range []string{secrets.DefaultPlaceholder, "[gone]", ""} {
				t.Run(field.name+": "+test.name+", placeholder "+placeholder, func(t *testing.T) {
					shown, _, _ := plantedRun(secrets.Setting{}, test, field)
					if got, want := field.get(shown[0]), test.value[:test.kept]+mark; got != want {
						t.Errorf("shown %q..., want %q...", tail(got), tail(want))
					}
					rows, sessions, _ := plantedRun(secrets.Setting{Withhold: true, Placeholder: placeholder}, test, field)
					want := test.value[:test.kept] + mark
					if test.withheld >= 0 {
						want = test.value[:test.withheld] + placeholder + mark
					}
					if got := field.get(rows[0]); got != want {
						t.Errorf("withheld %q, want %q", tail(got), tail(want))
					}
					for _, text := range writtenStrings(rows, sessions) {
						if strings.ContainsAny(text, "QX") {
							t.Errorf("an output string shows part of a secret: %q", tail(text))
						}
					}
				})
			}
		}
	}
}

func TestACutKeepsNoPartOfAShortSecretInTheTextOfItsParse(t *testing.T) {
	point := CarriedLimit - len(cutMark(10_000))
	value := filler(point-2) + fakeShort + filler(10_000-point+2-len(fakeShort))
	test := plantCase{"short", value, point, point - 2, fakeShort, recordBefore}
	rows, _, _ := plantedRun(secrets.Setting{Withhold: true, Placeholder: "P"}, test, plantedFields()[2])
	if want := "permission setting: " + value[:point-2] + "P" + cutMark(10_000) + ".leaf=1"; rows[0].Text != want {
		t.Errorf("text %q, want %q", tail(rows[0].Text), tail(want))
	}
}

// writtenStrings returns every string of rows and sessions.
func writtenStrings(rows []timeline.Row, sessions []timeline.SessionSummary) []string {
	var texts []string
	for _, row := range rows {
		texts = append(texts, row.Strings()...)
	}
	for _, session := range sessions {
		texts = append(texts, session.Host, session.User, session.SessionID, session.ProjectPath, session.SourceFile)
		texts = append(texts, session.Models...)
	}
	return texts
}

// tail returns the last 60 bytes of text, where a cut value differs.
func tail(text string) string {
	return text[max(0, len(text)-60):]
}

// FuzzACutShowsNoPartOfASecret holds that a value longer than the limit,
// holding a secret or a URL with a password at any place near the cut, in
// a field or in a row's text, is written with no byte of the secret under
// any placeholder, whenever the secret is identified, and holds exactly
// the URL credentials the whole value holds before the cut. The secret is in the
// letters Q, X, Z and W, which nothing else written holds; one in a field
// is at least secrets.EverywhereLength bytes, since a shorter one reaches
// only the text of its parse's rows.
func FuzzACutShowsNoPartOfASecret(fuzz *testing.F) {
	fuzz.Add(uint16(5000), uint8(45), "fake-secret-value", "REDACTED", uint8(0))
	fuzz.Add(uint16(4100), uint8(50), "secretpw", "", uint8(1|4|16))
	fuzz.Add(uint16(9000), uint8(0), "s", "[x]", uint8(2|8|32))
	fuzz.Fuzz(func(t *testing.T, length uint16, offset uint8, raw, placeholder string, shape uint8) {
		fields := plantedFields()
		field := fields[int(shape&3)%len(fields)]
		secret := planted(raw)
		if secret == "" || len(secret) > 64 || strings.ContainsAny(placeholder, "QXZW") ||
			field.name != "settings key" && len(secret) < secrets.EverywhereLength {
			return
		}
		value := plantedValue(int(length), int(offset), secret, shape)
		test := plantCase{value: value, secret: secret, when: recordBefore + recordAt(shape>>3&3)%3}
		switch {
		case shape&4 != 0:
			test.secret, test.when = "", recordNever
		case len(secret) < secrets.EverywhereLength && test.when == recordLater:
			// A short secret reaches only its own parse's rows.
			test.when = recordBefore
		}
		shown, _, _ := plantedRun(secrets.Setting{}, test, field)
		got := field.get(shown[0])
		if len(got) > CarriedLimit || len(value) > CarriedLimit && !strings.HasSuffix(got, cutMark(len(value))) {
			t.Fatalf("shown %d bytes ending %q", len(got), tail(got))
		}
		keepsTheURLCredentialsBefore(t, value, got)
		for _, mode := range []string{placeholder, ""} {
			rows, sessions, _ := plantedRun(secrets.Setting{Withhold: true, Placeholder: mode}, test, field)
			for _, text := range writtenStrings(rows, sessions) {
				if strings.ContainsAny(text, "QXZW") {
					t.Fatalf("placeholder %q: an output string shows part of the secret: %q", mode, tail(text))
				}
			}
		}
	})
}

// keepsTheURLCredentialsBefore fails the test unless written, a cut of
// value, holds exactly the URL credentials value holds wholly in the bytes
// it keeps, under either placeholder: none in part, and none value does
// not hold there.
func keepsTheURLCredentialsBefore(t *testing.T, value, written string) {
	t.Helper()
	kept := len(strings.TrimSuffix(written, cutMark(len(value))))
	for _, empty := range []bool{false, true} {
		var want [][2]int
		for _, each := range secrets.URLCredentialRanges(value, empty) {
			if each[0] < kept && each[1] > kept {
				t.Fatalf("the cut keeps part of the credential at %v", each)
			}
			if each[1] <= kept {
				want = append(want, each)
			}
		}
		if got := secrets.URLCredentialRanges(written, empty); !slices.Equal(got, want) {
			t.Fatalf("the cut holds the credentials %v, the whole value %v there", got, want)
		}
	}
}

// planted returns raw in the letters Q, X, Z and W, one per byte.
func planted(raw string) string {
	var out strings.Builder
	for i := range len(raw) {
		out.WriteByte("QXZW"[raw[i]%4])
	}
	return out.String()
}

// plantedValue returns a value of at least CarriedLimit+1 bytes holding,
// offset bytes after 128 before the cut point, the secret alone or, when
// shape has bit 4, a URL whose password it is, white space before it when
// shape has bit 32.
func plantedValue(length, offset int, secret string, shape uint8) string {
	length = max(length, CarriedLimit+1+len(secret)+40)
	planted := secret
	if shape&4 != 0 {
		planted = "https://Q:" + secret + "@h/"
	}
	if shape&32 != 0 {
		planted = " " + planted
	}
	place := CarriedLimit - len(cutMark(length)) - 128 + offset
	value := filler(place) + planted
	return value + filler(max(0, length-len(value)))
}

func TestACarriedCutPutIntoAFieldIsLeftOutOfTheFieldsCutWhole(t *testing.T) {
	point := CarriedLimit - len(cutMark(10_000))
	value := filler(point-12) + fakeSecret + filler(10_000-(point-12)-len(fakeSecret))
	for _, prefix := range []string{"pppppp", "ppp"} {
		for _, placeholder := range []string{secrets.DefaultPlaceholder, ""} {
			t.Run("prefix "+prefix+", placeholder "+placeholder, func(t *testing.T) {
				var length int
				produce := func(artifact *input.Artifact, emit Emit) {
					SecretText(artifact, fakeSecret)
					field := prefix + NewCarried(artifact).Cut("settings key", value) + "suffix-suffix-suffix-suffix"
					length = len(field)
					emit(timeline.Row{SessionID: field, TurnType: timeline.TurnSystem, Text: "x"})
				}
				_, _, shown := carriedRun(secrets.Setting{}, produce, nil)
				if want := prefix + cutMark(length); shown[0].SessionID != want {
					t.Errorf("shown %q, want %q", tail(shown[0].SessionID), want)
				}
				rows, sessions, _ := carriedRun(secrets.Setting{Withhold: true, Placeholder: placeholder}, produce, nil)
				for _, text := range writtenStrings(rows, sessions) {
					if strings.ContainsAny(text, "QX") {
						t.Errorf("an output string shows part of the secret: %q", tail(text))
					}
				}
			})
		}
	}
}

func TestEarlierCutsAreTheBoundBeforeEachMarkShapedText(t *testing.T) {
	long := strings.Repeat("a", CarriedLimit)
	tests := []struct {
		name  string
		value string
		want  [][2]int
	}{
		{"no mark", "plain text", nil},
		{"a mark near the start reaches back to it", "ab [cut from 12 bytes] cd", [][2]int{{0, 22}}},
		{"a mark reaches back the limit before its end", long + " [cut from 9 bytes]",
			[][2]int{{19, CarriedLimit + 19}}},
		{"a mark's reach starts at a character", "é" + long[:CarriedLimit-20] + " [cut from 9 bytes]",
			[][2]int{{0, CarriedLimit + 1}}},
		{"a mark without digits is no mark", "ab [cut from bytes] [cut from x bytes]", nil},
		{"each of two marks", "a [cut from 1 bytes] [cut from 22 bytes]", [][2]int{{0, 20}, {0, 40}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := earlierCuts(test.value); !slices.Equal(got, test.want) {
				t.Errorf("earlierCuts = %v, want %v", got, test.want)
			}
		})
	}
}

func TestALongProjectPathIsCutBeforeTheURLItsSpellingKeepsAsWritten(t *testing.T) {
	point := CarriedLimit - len(cutMark(10_000))
	head := `C:\p\https://h/`
	value := head + filler(point-5-len(head)) + fakeSecret
	value += filler(10_000-len(value)-len("?token=QXQXQXQXQX")) + "?token=QXQXQXQXQX"
	for _, placeholder := range []string{secrets.DefaultPlaceholder, ""} {
		t.Run("placeholder "+placeholder, func(t *testing.T) {
			produce := func(artifact *input.Artifact, emit Emit) {
				SecretText(artifact, fakeSecret)
				emit(timeline.Row{ProjectPath: value, TurnType: timeline.TurnUser, Text: "x"})
			}
			_, _, shown := carriedRun(secrets.Setting{}, produce, nil)
			got := shown[0].ProjectPath
			if want := `C:\p\` + cutMark(10_000); got != want || timeline.CanonicalProjectPath(got) != got {
				t.Errorf("project path %q, want %q, its own spelling", tail(got), want)
			}
			rows, sessions, _ := carriedRun(secrets.Setting{Withhold: true, Placeholder: placeholder}, produce, nil)
			for _, text := range writtenStrings(rows, sessions) {
				if strings.ContainsAny(text, "QX") {
					t.Errorf("an output string shows part of a secret: %q", tail(text))
				}
			}
		})
	}
}
