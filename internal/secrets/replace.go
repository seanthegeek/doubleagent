package secrets

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textset"
)

// Text returns text, one string the output writes, as the run shows it: as
// written, or with the credentials of the URLs in it, the secrets of scopes
// (the parses whose rows it comes from, or the command's record) and every
// secret of EverywhereLength bytes or more replaced by the placeholder.
// Secrets that overlap are one replaced range (replaceRanges), so a secret
// inside a longer one (a password inside user:password) goes with it. The output is
// built from the original and the placeholder, never read again. The cost
// is the text's length once per scope and once for everywhere, after each
// record's matcher is built once.
func (r *Recorder) Text(text string, scopes ...*Found) string {
	if r == nil || text == "" {
		return text
	}
	sources := r.sources(scopes)
	if mayHoldURL(text) {
		inText := &Found{}
		identifyURLs(text, inText.add, r.work)
		if len(inText.all) > 0 {
			// The credentials of a URL are replaced in the string that holds
			// it whatever their length, so this record is matched in full.
			inText.local = inText.all
			sources = append(sources, inText)
		}
	}
	if len(sources) == 0 {
		return text
	}
	longest := make([]int, len(text))
	for _, source := range sources {
		source.matcherOf(source == r.everywhere, r.setting.Placeholder == "", r.work).Longest(text, longest)
	}
	return replaceRanges(text, longest, r.setting.Placeholder)
}

// replaceRanges returns text with each range of secrets replaced by
// placeholder, where longest[i] is the length of the longest secret that
// starts at byte i. Secrets that share a byte are one range, so a secret
// that begins inside another and ends after it goes whole, and two that
// only touch are two ranges. Each byte is looked at once.
func replaceRanges(text string, longest []int, placeholder string) string {
	var out strings.Builder
	written := 0
	for i := 0; i < len(text); {
		if longest[i] == 0 {
			i++
			continue
		}
		end := i + longest[i]
		for k := i + 1; k < end; k++ {
			end = max(end, k+longest[k])
		}
		out.WriteString(text[written:i])
		out.WriteString(placeholder)
		i, written = end, end
	}
	if written == 0 {
		return text
	}
	out.WriteString(text[written:])
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
