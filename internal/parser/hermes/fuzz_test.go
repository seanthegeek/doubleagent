package hermes

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

func FuzzConfigAndCronReaders(fuzz *testing.F) {
	for _, seed := range []string{"command_allowlist:\n  - 'a''b'\n  - \"c\\u00e9\" # x\n",
		"**Prompt Characters:** 5\n## Prompt\n\nhéllo\n\n**Response Characters:** 3\n## Response\n\nwör\n",
		"## Prompt\n\n## Response\n\n", `"a\`} {
		fuzz.Add(seed)
	}
	subject := New()
	fuzz.Fuzz(func(t *testing.T, text string) {
		_, _ = subject.readFlatList(textlines.Split(text), "command_allowlist")
		_, _ = subject.yamlScalar(text)
		_, _, _ = subject.splitCronDocument(text)
	})
}
