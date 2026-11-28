package cody

import (
	"strings"
	"testing"
)

func TestAUserRowNamesTheContextFilesOfEitherForm(t *testing.T) {
	const jetBrains = "Cody-nodejs/JetBrains-globalState/cody-local-chatHistory-v2"
	uri := func(path string) string { return `{"fsPath":"` + path + `"}` }
	tests := []struct {
		name, human, beside, want string
	}{
		{"fileName alone", `"text":"q"`, `"usedContextFiles":[{"fileName":"src/a.py"}]`, "q\n[context: src/a.py]"},
		{"fileName and uri", `"text":"q"`, `"usedContextFiles":[{"fileName":"src/a.py","uri":` + uri("/h/src/a.py") + `}]`,
			"q\n[context: /h/src/a.py]"},
		{"uri alone", `"text":"q"`, `"usedContextFiles":[{"uri":` + uri("/h/b.py") + `}]`, "q\n[context: /h/b.py]"},
		{"contextFiles only", `"text":"q","contextFiles":[{"uri":` + uri("/h/c.py") + `}]`, ``, "q\n[context: /h/c.py]"},
		{"both members", `"text":"q","contextFiles":[{"uri":` + uri("/h/c.py") + `}]`,
			`"usedContextFiles":[{"fileName":"d.py"}]`, "q\n[context: /h/c.py, d.py]"},
		{"neither", `"text":"q"`, ``, "q"},
		{"not a list", `"text":"q"`, `"usedContextFiles":"x"`, "q"},
		{"a non-object entry", `"text":"q"`, `"usedContextFiles":["x",3,{"fileName":"e.py"}]`, "q\n[context: e.py]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			interaction := `{"humanMessage":{"speaker":"human",` + test.human + `},"assistantMessage":{"speaker":"assistant","text":"ok"}`
			if test.beside != "" {
				interaction += "," + test.beside
			}
			history := `{"https://x.example-u":{"chat":{"c":{"id":"c","chatTitle":"t","interactions":[` + interaction + `}]}}}}`
			got := parseFile(t, jetBrains, []byte(history))
			var user string
			for _, text := range got {
				if strings.HasPrefix(text, "q") {
					user = text
				}
			}
			if user != test.want {
				t.Errorf("user row = %q, want %q (all rows %q)", user, test.want, got)
			}
		})
	}
}
