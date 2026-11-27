package parserkit

import (
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
)

// kindsParser is a parser that lists two kinds.
type kindsParser struct{ Base }

// Kinds lists a file by its path and a pattern.
func (kindsParser) Kinds() []Kind {
	return []Kind{RelKind("history", "a/history.jsonl"), PatternKind("session", MustRelPattern(`\Aa/s/[^/]+\.jsonl\z`))}
}

// Wants wants what its kinds want.
func (p kindsParser) Wants(artifact *input.Artifact) bool { return WantsAny(p.Kinds(), artifact) }

// Parse emits nothing.
func (kindsParser) Parse(*input.Artifact, Options, Emit) error { return nil }

// Each kind wants the artifacts its rule names in any ASCII case, and
// WantsAny wants those of any kind, trying the kinds in order and stopping at the first that
// wants the artifact.
func TestKindsWantTheArtifactsTheirRulesName(t *testing.T) {
	tests := []struct {
		rel  string
		want []string
	}{
		{"a/history.jsonl", []string{"history"}},
		{"a/s/one.jsonl", []string{"session"}},
		{"A/History.JSONL", []string{"history"}},
		{"A/S/One.Jsonl", []string{"session"}},
		{"a/hi\u017ftory.jsonl", nil},
		{"a/s/deeper/one.jsonl", nil},
		{"b/history.jsonl", nil},
	}
	subject := kindsParser{NewBase("agent", "agent")}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Rel: test.rel}
			var got []string
			for _, kind := range subject.Kinds() {
				if kind.Wants(artifact) {
					got = append(got, kind.Name)
				}
			}
			if !slices.Equal(got, test.want) || subject.Wants(artifact) != (len(test.want) > 0) {
				t.Errorf("kinds %v want it, Wants says %v; want %v", got, subject.Wants(artifact), test.want)
			}
		})
	}
	var tried []string
	kinds := []Kind{RelFuncKind("first", func(rel string) bool { tried = append(tried, "first"); return true }),
		RelFuncKind("second", func(rel string) bool { tried = append(tried, "second"); return true })}
	if !WantsAny(kinds, &input.Artifact{Rel: "x"}) || !slices.Equal(tried, []string{"first"}) {
		t.Errorf("WantsAny tried %v, want the first kind alone", tried)
	}
}
