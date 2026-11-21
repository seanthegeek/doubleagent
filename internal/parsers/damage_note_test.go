package parsers

import (
	"slices"
	"testing"
)

func TestADamagedFileWhoseRecoveredPartRepeatsANameKeepsItsDamageRow(t *testing.T) {
	tests := []struct {
		name, parser, rel, data, damage string
	}{
		{"a Roo Code CLI history with text after its entries", "roo-code", ".roo/cli-history.json",
			`{"entries":["a",{"x":1,"x":2},"b"]} junk`, "parser: cli-history.json unparseable after its entries array"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifacts, _ := growthHome(t, test.parser, one(test.rel, []byte(test.data)))
			var texts []string
			repeats := 0
			for _, row := range rowsOf(t, newParser(t, test.parser), artifacts) {
				texts = append(texts, row.Text)
				if isRepeatRow(row) {
					repeats++
				}
			}
			if !slices.Contains(texts, test.damage) || repeats != 1 {
				t.Errorf("rows %q, want %q and one repeat row", texts, test.damage)
			}
		})
	}
}
