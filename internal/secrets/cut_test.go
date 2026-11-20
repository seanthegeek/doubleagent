package secrets

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestACutPointKeepsWholeCharactersAndNoPartOfAURL(t *testing.T) {
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
		{"a token holding a URL is left out whole", "see x/https://u:p@h/", 15, 4},
		{"a token holding a URL with escaped slashes is left out whole", `see https:\/\/u:p@h`, 14, 4},
		{"a value that is one token holding a URL keeps nothing", "https://u:password@host", 15, 0},
		{"a token whose separator the limit cuts through is cut at the limit", "see https://u:p@h", 10, 10},
		{"a URL after the limit does not move the cut", "plain text https://u:p@h", 9, 9},
		{"a token before the URL's is kept whole", "keep\thttps://u:p@h", 14, 5},
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
