package parserfacts

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// module is the module path the synthetic parser imports the kit from.
const module = "github.com/seanthegeek/doubleagent"

// alphaPath is the import path of the synthetic parser package.
const alphaPath = "example.test/alpha"

// alphaSource is a parser written with the kit, holding one string of each
// kind the analysis looks for and strings it must not take for one.
const alphaSource = `package alpha

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// A comment naming "commentKey" and "comment text: " is no fact.

var logPattern = parserkit.MustRelPattern(` + "`" + `\A\.alpha/logs/[^/]+\.jsonl\z` + "`" + `)

var historyPattern = parserkit.MustRelPattern(` + "`" + `\A\.alpha/history\.jsonl\z` + "`" + `)

const settingsRel = ".alpha/settings.json"

type Parser struct {
	parserkit.Base
	prefix string
}

func New() *Parser { return &Parser{Base: parserkit.NewBase("alpha", "alpha"), prefix: ".alpha"} }

func (p *Parser) Wants(artifact *input.Artifact) bool { return parserkit.WantsAny(p.Kinds(), artifact) }

func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.PatternKind("log", logPattern), parserkit.RelKind("settings", settingsRel),
		parserkit.RelFuncKind("history", func(rel string) bool { return historyPattern.MatchString(rel) }),
		parserkit.RelKind("state", p.prefix+"/state.db")}
}

func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return historyPattern.MatchString(artifact.Rel) || textlines.EqualFoldASCII(".alpha/old.jsonl", artifact.Rel) ||
		textlines.HasSuffixFoldASCII(artifact.Rel, "/copy.jsonl")
}

func (p *Parser) Parse(artifact *input.Artifact, _ parserkit.Options, emit parserkit.Emit) error {
	var problems parserkit.Problems
	record := jsonvalue.NewObject()
	kind := record.Get("directKey").Text()
	emit(p.SystemRow(artifact, "model: "+kind))
	emit(p.SystemRow(artifact, field(record, "helperKey")))
	for _, name := range []string{"rangedKey"} {
		_ = record.Has(name)
	}
	row := p.BaseRow(artifact)
	row.Text = "permission decision: " + strings.ToUpper("notAKey")
	emit(row)
	emit(timeline.Row{Text: label("session synced: ")})
	problems.Add(1, "unparseable line (no header)")
	emit(p.SystemRow(artifact, parserkit.ProblemPrefix+"record "+kind+" has no id"))
	_ = parserkit.ArrayMember(record.Get("memberKey"), "memberKey", 1, &problems)
	_ = strings.Repeat("unrelatedCall", 1)
	return nil
}

func field(record jsonvalue.Value, key string) string { return record.Get(key).Text() }

func label(text string) string { return text + "x" }
`

// analyzeAlpha writes the synthetic parser under a temporary directory
// and returns its report.
func analyzeAlpha(t *testing.T) report {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "alpha.go"), []byte(alphaSource), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	config := settings{root: root, module: module, tags: []string{"noasm", "nounsafe"},
		overlays: map[string]string{alphaPath: dir},
		isAgent:  func(path string) bool { return path == alphaPath }}
	found, err := analyze(config, []target{{Agent: "alpha", Package: alphaPath, TypeName: "Parser"}})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// factTexts returns the texts of a report's facts of one kind, or for
// textFact their constant starts.
func factTexts(found report, kind factKind) []string {
	var texts []string
	for _, each := range found.Facts {
		if each.Kind == kind && kind == textFact {
			texts = append(texts, each.Prefix)
		} else if each.Kind == kind {
			texts = append(texts, each.Text)
		}
	}
	return texts
}

// patternTexts returns a report's first parser's patterns, those its
// Secondary method tests when secondary is set, else those of its kinds.
func patternTexts(found report, secondary bool) []string {
	var texts []string
	for _, each := range found.Parsers[0].Patterns {
		if each.Secondary == secondary && each.Resolved {
			texts = append(texts, each.Text)
		}
	}
	return texts
}

// sourceLine returns the line of alphaSource holding text.
func sourceLine(text string) int {
	return strings.Count(alphaSource[:strings.Index(alphaSource, text)], "\n") + 1
}

func TestTheAnalysisOfAParser(t *testing.T) {
	found := analyzeAlpha(t)
	facts := []struct {
		name    string
		kind    factKind
		present []string
		absent  []string
	}{
		{"finds a key read directly, through a helper, from a range and by a kit wrapper, and no other string",
			keyFact, []string{"directKey", "helperKey", "rangedKey", "memberKey"},
			[]string{"commentKey", "notAKey", "unrelatedCall", "alpha"}},
		{"finds a problem message and a parser: row text, and no message the kit words",
			messageFact, []string{"unparseable line (no header)", "record {} has no id"},
			[]string{"memberKey is not a JSON array", "memberKey"}},
		{"finds each row text's constant start wherever the text is built, and no comment",
			textFact, []string{"model: ", "permission decision: ", "session synced: "},
			[]string{"comment text: ", "parser: record "}},
	}
	for _, each := range facts {
		t.Run(each.name, func(t *testing.T) {
			t.Parallel()
			texts := factTexts(found, each.kind)
			for _, want := range each.present {
				if !slices.Contains(texts, want) {
					t.Errorf("%s %q not found in %q", each.kind, want, texts)
				}
			}
			for _, unwanted := range each.absent {
				if slices.Contains(texts, unwanted) {
					t.Errorf("%s %q found in %q", each.kind, unwanted, texts)
				}
			}
		})
	}
	t.Run("gives the kinds' patterns and the secondary tests, each resolved", func(t *testing.T) {
		t.Parallel()
		want := []string{`\A\.alpha/logs/[^/]+\.jsonl\z`, ".alpha/settings.json",
			`\A\.alpha/history\.jsonl\z`, ".alpha/state.db"}
		if kinds := patternTexts(found, false); !slices.Equal(kinds, want) {
			t.Errorf("kind patterns are %q, want %q", kinds, want)
		}
		wantSecondary := []string{want[2], ".alpha/old.jsonl", "/copy.jsonl"}
		if secondary := patternTexts(found, true); !slices.Equal(secondary, wantSecondary) {
			t.Errorf("secondary patterns are %q, want %q", secondary, wantSecondary)
		}
	})
	t.Run("places each fact at the file and line of its constant", func(t *testing.T) {
		t.Parallel()
		for _, each := range found.Facts {
			if each.Text == "directKey" && (filepath.Base(each.File) != "alpha.go" ||
				each.Line != sourceLine(`"directKey"`)) {
				t.Errorf("directKey is placed at %s:%d, want alpha.go:%d", each.File, each.Line,
					sourceLine(`"directKey"`))
			}
		}
	})
}
