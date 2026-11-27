package gitmeta

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

// mixedConfig is one config text of testdata/configs.json, built from
// fragments of headers, keys, separators, quotes, escapes, comments and line
// ends in combinations the hand-written cases do not reach, with the remotes,
// rejected values and HEAD value read from it.
type mixedConfig struct {
	Name    string `json:"name"`
	Config  string `json:"config"`
	Remotes []struct {
		Name     string   `json:"name"`
		URLs     []string `json:"urls"`
		PushURLs []string `json:"pushurls"`
	} `json:"remotes"`
	Problems []string `json:"problems"`
	Head     string   `json:"head"`
}

// loadMixedConfigs reads testdata/configs.json.
func loadMixedConfigs(t testing.TB) []mixedConfig {
	t.Helper()
	data, err := os.ReadFile("testdata/configs.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []mixedConfig
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func TestMixedConfigs(t *testing.T) {
	for _, test := range loadMixedConfigs(t) {
		t.Run(test.Name, func(t *testing.T) {
			want := []Remote{}
			for _, found := range test.Remotes {
				want = append(want, Remote{Name: found.Name, URLs: found.URLs,
					PushURLs: found.PushURLs})
			}
			remotes, problems := ParseConfig(test.Config)
			if !reflect.DeepEqual(remotes, want) {
				t.Errorf("ParseConfig(%q) remotes:\n%+v\nwant:\n%+v", test.Config, remotes, want)
			}
			if !slices.Equal(problems, test.Problems) {
				t.Errorf("ParseConfig(%q) problems %q, want %q", test.Config, problems,
					test.Problems)
			}
			if got := ParseHead(test.Config); got != test.Head {
				t.Errorf("ParseHead(%q) = %q, want %q", test.Config, got, test.Head)
			}
		})
	}
}
