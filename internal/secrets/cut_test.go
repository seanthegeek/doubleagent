package secrets

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestACutPointKeepsWholeCharactersAndNoPartOfAURLCredential(t *testing.T) {
	long := strings.Repeat("x", 40)
	tests := []struct {
		name  string
		value string
		limit int
		want  int
	}{
		{"a value within the limit is kept whole", "abc", 5, 3},
		{"plain text is cut at the limit", "abcdefgh", 5, 5},
		{"a character across the limit is left out whole", "abcdé", 5, 4},
		{"a four-byte character across the limit is left out whole", "ab\U0001F600cd", 4, 2},
		{"a URL without credentials is cut at the limit", "https://a.example/" + long, 30, 30},
		{"a URL of another scheme is cut at the limit", "s3://bucket/" + long, 30, 30},
		{"user information across the limit is left out from its start", "see x/https://u:p@h/", 15, 14},
		{"user information with escaped slashes is left out from its start", `see https:\/\/u:pw@h`, 15, 14},
		{"user information whose @ the cut would leave out is left out", "https://u:password@host", 18, 8},
		{"user information kept with its @ is kept", "https://u:password@host/" + long[:10], 19, 19},
		{"a scheme the cut runs through is left out", "see https://u:p@h", 10, 4},
		{"a URL after the limit does not move the cut", "plain text https://u:p@h", 9, 9},
		{"a query value across the limit is left out from its start", "x https://h/?token=abcdef&a=b", 21, 19},
		{"a cut right after a query value moves back to its start", "x https://h/?token=abc.&a=b", 23, 19},
		{"a cut at a query value's start keeps its name", "x https://h/?token=abcdef", 19, 19},
		{"a parameter that holds no credential is cut at the limit", "x https://h/?page=abcdef", 21, 21},
		{"a name-only user of fewer than eight bytes is cut at the limit", "https://git@h/", 10, 10},
		{"a token running far past the limit keeps its URL's scheme only", "a https://u:p@h/" + long + long, 20,
			10},
		{"a token running far past the limit with escaped slashes keeps its scheme", `a https:\/\/h/` + long + long, 20,
			12},
		{"a token running far past the limit with no URL in its kept part is cut at the limit", "a " + long +
			"https://u:p@h/" + long + long, 20, 20},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CutPoint(test.value, test.limit); got != test.want {
				t.Errorf("CutPoint(%q, %d) = %d, want %d", test.value, test.limit, got, test.want)
			}
		})
	}
}

func TestACutValueHoldsOnlyTheURLCredentialsOfTheWholeValue(t *testing.T) {
	values := []string{
		"a https://u:pw@h/ b https://v:pw@h/?token=abcdefgh c",
		"x?token=abc,https://u:p@h/ tail",
		"https://a:b@c@d/ e https://f@g/",
		`q https:\/\/u:password@h\/ r`,
	}
	for _, value := range values {
		for limit := range len(value) + 1 {
			point := CutPoint(value, limit)
			kept := URLCredentialRanges(value[:point]+" [cut]", false)
			var whole [][2]int
			for _, each := range URLCredentialRanges(value, false) {
				if each[1] <= point {
					whole = append(whole, each)
				}
				if each[0] < point && each[1] > point {
					t.Errorf("%q cut at %d (limit %d) keeps part of the credential at %v", value, point, limit, each)
				}
			}
			if !slices.Equal(kept, whole) {
				t.Errorf("%q cut at %d holds the credentials %v, the whole value %v there", value, point, kept, whole)
			}
		}
	}
}

func TestACutWithholdsTheKeptPartOfASecretWhereverTheWrittenTextStands(t *testing.T) {
	value := strings.Repeat("a", 20) + "fake-secret-0001" + strings.Repeat("b", 20)
	written := value[:24] + " [cut]"
	tests := []struct {
		name    string
		record  func(recorder *Recorder)
		inText  bool
		field   string
		rowText string
	}{
		{"a long secret is withheld in every string", func(recorder *Recorder) { recorder.Value("fake-secret-0001") },
			false, strings.Repeat("a", 20) + "P [cut]", "x " + strings.Repeat("a", 20) + "P [cut] y"},
		{"a long URL credential of another string is withheld too", func(recorder *Recorder) {
			recorder.Collect("https://fake-secret-0001@h/")
		}, false, strings.Repeat("a", 20) + "P [cut]", "x " + strings.Repeat("a", 20) + "P [cut] y"},
		{"a short form of the parse is withheld wherever the written text stands", func(recorder *Recorder) {
			recorder.RowValue("fake-secret-0001")
		}, true, strings.Repeat("a", 20) + "P [cut]", "x " + strings.Repeat("a", 20) + "P [cut] y"},
		{"nothing recorded leaves the written text", func(*Recorder) {}, false, written, "x " + written + " y"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := NewRecorder(Setting{Withhold: true, Placeholder: "P"})
			found := recorder.Begin()
			recorder.Cut(value, written, 24, test.inText)
			test.record(recorder)
			recorder.End()
			if got := recorder.Text(written); got != test.field {
				t.Errorf("field %q, want %q", got, test.field)
			}
			if got := recorder.RowText("x "+written+" y", found); got != test.rowText {
				t.Errorf("row text %q, want %q", got, test.rowText)
			}
		})
	}
}

func TestACutValueIsRecordedOnceAndItsDroppedURLCredentialsEverywhere(t *testing.T) {
	recorder := NewRecorder(withheld())
	value := "keep https://fake-user:fake-password-0001@h/"
	for range 3 {
		recorder.Cut(value, "keep  [cut]", 5, false)
	}
	if got := recorder.CutValues(); len(got) != 1 || got[0] != (CutValue{value, "keep  [cut]", 5, nil}) {
		t.Errorf("cut values %+v, want one", got)
	}
	if got := recorder.Text("seen fake-user:fake-password-0001"); got != "seen REDACTED" {
		t.Errorf("a dropped credential is written %q elsewhere", got)
	}
	var shown *Recorder
	shown.Cut(value, "keep  [cut]", 5, false)
	if shown.CutValues() != nil || shown.Text("keep  [cut]") != "keep  [cut]" {
		t.Error("a run that shows secrets changed what it writes")
	}
}

// cutWork returns the work of cutting n values of 6,000 bytes, each kept
// to its first 4,000, then collecting and writing each written text, by
// the name of the work: with every written text the same or each its own,
// and with a recorded secret in each value running across its cut or none.
func cutWork(t *testing.T, n int, shared, withSecrets bool) counter {
	t.Helper()
	const length, kept = 6000, 4000
	work := counter{}
	recorder := NewRecorder(withheld())
	recorder.work = work
	common := strings.Repeat("abcdefghi/", length/10)
	written := make([]string, n)
	for i := range n {
		head := common[:kept-5]
		if !shared {
			head = fmt.Sprintf("%06d", i) + common[6:kept-5]
		}
		secret := fmt.Sprintf("QXQXQ%06dQXQXQ", i)
		value := head + secret + common[kept+len(secret)-5:]
		if withSecrets {
			recorder.Value(secret)
		}
		written[i] = value[:kept] + " [cut]"
		recorder.Cut(value, written[i], kept, false)
	}
	for _, text := range written {
		recorder.Collect(text)
	}
	for _, text := range written {
		if got := recorder.Text(text); withSecrets && strings.Contains(got, "QX") {
			t.Fatalf("a written text shows part of a secret: %q", got[len(got)-20:])
		}
	}
	return work
}

func TestCuttingAndWritingManyValuesCostsTheirLengthAFixedNumberOfTimes(t *testing.T) {
	tests := []struct {
		name                string
		shared, withSecrets bool
	}{
		{"values with one written text", true, false},
		{"values each with its own written text", false, false},
		{"values with one written text and a secret across each cut", true, true},
		{"values each with its own written text and a secret across each cut", false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			small, double, quadruple := cutWork(t, 250, test.shared, test.withSecrets),
				cutWork(t, 500, test.shared, test.withSecrets), cutWork(t, 1000, test.shared, test.withSecrets)
			// Every value costs the same, so doubling the values doubles
			// what they cost, exactly, whatever their written texts share.
			for _, name := range []string{"cut bytes compared", "bytes read", "bytes indexed", "bytes searched",
				"fallbacks taken"} {
				if quadruple[name]-double[name] != 2*(double[name]-small[name]) {
					t.Errorf("%s: %d, %d and %d for 250, 500 and 1000 values: not linear", name, small[name],
						double[name], quadruple[name])
				}
			}
			if test.withSecrets && double["bytes searched"] <= small["bytes searched"] {
				t.Errorf("searched %d and %d bytes: the written texts were not searched", small["bytes searched"],
					double["bytes searched"])
			}
		})
	}
}

// cutPieces are what a value of FuzzACutKeepsExactlyTheURLCredentialsBefore
// is built of: the bytes each rule of the URL grammar reads, schemes,
// credential names, a two-byte character and filler.
func cutPieces() []string {
	return []string{"https://", `s3:\/\/`, "x", "u", ":", "@", "/", `\`, "?", "#", "&", ";", "=", "token=",
		"key=", "page=", ",", ".", "!", "'", "(", ")", "]", " ", `"`, `&`, "é", "password", "ab", "ftp"}
}

// referenceCutPoint returns where a cut at point ends up by the rule as
// stated: moved back to the start of each span it lies inside, one at a
// time, until it lies inside none.
func referenceCutPoint(point int, spans [][2]int) int {
	for moved := true; moved; {
		moved = false
		for _, span := range spans {
			if span[0] < point && point < span[1] {
				point, moved = span[0], true
			}
		}
	}
	return point
}

// FuzzACutKeepsExactlyTheURLCredentialsBefore holds that a value cut at
// CutPoint, with white space and a mark after the kept bytes, holds exactly
// the URL credentials the whole value holds wholly before the cut, under
// either placeholder: no part of one across the cut, and none the whole
// value does not hold there. It also holds that the cut point is the one
// the rule as stated gives.
func FuzzACutKeepsExactlyTheURLCredentialsBefore(fuzz *testing.F) {
	fuzz.Add([]byte{0, 3, 4, 5, 6, 2, 3, 7, 2}, uint8(9))
	fuzz.Add([]byte{2, 0, 7, 8, 13, 3, 16, 9, 2, 2, 2, 2}, uint8(20))
	fuzz.Add([]byte{1, 3, 4, 27, 5, 2, 8, 13, 28, 25, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}, uint8(12))
	pieces := cutPieces()
	fuzz.Fuzz(func(t *testing.T, raw []byte, limit uint8) {
		var built strings.Builder
		for _, each := range raw {
			built.WriteString(pieces[int(each)%len(pieces)])
		}
		value := built.String()
		point := CutPoint(value, int(limit))
		start := min(int(limit), len(value))
		for start > 0 && start < len(value) && !utf8.RuneStart(value[start]) {
			start--
		}
		if start < len(value) {
			if want := referenceCutPoint(start, cutSpans(value, start, int(limit))); point != want {
				t.Fatalf("CutPoint(%q, %d) = %d, the rule gives %d", value, limit, point, want)
			}
		}
		kept := value[:point] + " [cut from 99 bytes]"
		for _, empty := range []bool{false, true} {
			var want [][2]int
			for _, each := range URLCredentialRanges(value, empty) {
				if each[0] < point && each[1] > point {
					t.Fatalf("%q cut at %d keeps part of the credential at %v", value, point, each)
				}
				if each[1] <= point {
					want = append(want, each)
				}
			}
			if got := URLCredentialRanges(kept, empty); !slices.Equal(got, want) {
				t.Fatalf("%q cut at %d holds the credentials %v, the whole value %v there", value, point, got, want)
			}
		}
	})
}
