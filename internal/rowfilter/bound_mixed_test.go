package rowfilter

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// mixedBound is one value of testdata/bounds.json: a value in an accepted
// form, some with fields out of range, or such a value with a few bytes
// replaced, inserted or deleted, read at a clock between the years 1900 and
// 2100. Since and Until are empty for a value that is refused.
type mixedBound struct {
	Name  string `json:"name"`
	Text  string `json:"text"`
	Now   string `json:"now"`
	Since string `json:"since"`
	Until string `json:"until"`
}

// loadMixedBounds reads testdata/bounds.json.
func loadMixedBounds(t testing.TB) []mixedBound {
	t.Helper()
	data, err := os.ReadFile("testdata/bounds.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []mixedBound
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func TestMixedBounds(t *testing.T) {
	for _, test := range loadMixedBounds(t) {
		t.Run(test.Name, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339Nano, test.Now)
			if err != nil {
				t.Fatal(err)
			}
			for _, bound := range []struct {
				upper bool
				want  string
			}{{false, test.Since}, {true, test.Until}} {
				got, err := ParseBound(test.Text, bound.upper, now)
				var boundErr *BoundError
				switch {
				case bound.want == "" && !errors.As(err, &boundErr):
					t.Errorf("ParseBound(%q, upper %v) at %s = %q, %v; want a BoundError",
						test.Text, bound.upper, test.Now, got, err)
				case bound.want != "" && (err != nil || got != bound.want):
					t.Errorf("ParseBound(%q, upper %v) at %s = %q, %v; want %q", test.Text,
						bound.upper, test.Now, got, err, bound.want)
				}
			}
		})
	}
}

func FuzzParseBound(fuzz *testing.F) {
	for _, test := range loadMixedBounds(fuzz) {
		fuzz.Add(test.Text)
	}
	fuzz.Fuzz(func(t *testing.T, text string) {
		since, sinceErr := ParseBound(text, false, testNow())
		until, untilErr := ParseBound(text, true, testNow())
		if (sinceErr == nil) != (untilErr == nil) {
			t.Fatalf("ParseBound(%q) reads one bound only: %v, %v", text, sinceErr, untilErr)
		}
		if sinceErr != nil {
			var boundErr *BoundError
			if !errors.As(sinceErr, &boundErr) || !strings.HasSuffix(sinceErr.Error(), FormsHelp) {
				t.Fatalf("ParseBound(%q) error %v is not a BoundError with the forms", text,
					sinceErr)
			}
			return
		}
		if since > until {
			t.Fatalf("ParseBound(%q) gives since %q after until %q", text, since, until)
		}
	})
}
