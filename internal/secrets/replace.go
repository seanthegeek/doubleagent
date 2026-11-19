package secrets

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textset"
)

// Text returns text, one string the output writes, as the run shows it: as
// written, or with the secrets of scopes (the parses whose rows it comes
// from, or the command's record) and every secret of EverywhereLength bytes
// or more replaced by the placeholder, the one place it is written. Text is
// read once, left to right, the longest recorded text starting at each
// position replaced and reading going on after it, so a secret inside a
// longer one (a password inside user:password) goes with it. The output is
// built from the original and the placeholder, never read again. The cost
// is the text's length once per scope and once for everywhere, after each
// record's matcher is built once.
func (r *Recorder) Text(text string, scopes ...*Found) string {
	if r == nil || text == "" {
		return text
	}
	sources := r.sources(scopes)
	if len(sources) == 0 {
		return text
	}
	longest := make([]int, len(text))
	for _, source := range sources {
		source.matcherOf(source == r.everywhere, r.setting.Placeholder == "", r.work).Longest(text, longest)
	}
	var out strings.Builder
	start := 0
	for i := 0; i < len(text); {
		if longest[i] == 0 {
			i++
			continue
		}
		out.WriteString(text[start:i])
		out.WriteString(r.setting.Placeholder)
		i += longest[i]
		start = i
	}
	if start == 0 {
		return text
	}
	out.WriteString(text[start:])
	return out.String()
}

// sources returns everywhere and the distinct scopes that hold a secret
// they would replace, or none when no record holds one.
func (r *Recorder) sources(scopes []*Found) []*Found {
	var sources []*Found
	if len(r.everywhere.all) > 0 {
		sources = append(sources, r.everywhere)
	}
	for _, scope := range scopes {
		if scope != nil && len(scope.local) > 0 && !slices.Contains(sources, scope) {
			sources = append(sources, scope)
		}
	}
	return sources
}

// matcherOf returns the matcher of the record's texts (every text for
// everywhere, else those of short secrets; the forms with a part only for an
// empty placeholder), building it again only when texts were recorded since.
func (f *Found) matcherOf(everywhere, withParts bool, work textset.Counter) *textset.Set {
	texts := f.local
	if everywhere {
		texts = f.all
	}
	if f.matcher == nil || f.matcherTexts != len(texts) {
		var matched []string
		for _, text := range texts {
			if withParts || f.forms[text].plain() {
				matched = append(matched, text)
			}
		}
		f.matcher, f.matcherTexts = textset.NewBackward(matched, work), len(texts)
	}
	return f.matcher
}
