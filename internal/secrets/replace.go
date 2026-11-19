package secrets

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/secrets/urlcredentials"
	"github.com/seanthegeek/doubleagent/internal/textset"
)

// Text returns text, one string the output writes that is neither the
// text of a row nor a problem line (a field of a row or a session, a key or
// value of inventory.json, a line of a summary or an error), as the run
// shows it: as written, or with the credentials of the URLs in it and every
// secret of EverywhereLength bytes or more replaced by the placeholder.
func (r *Recorder) Text(text string) string {
	return r.replace(text, nil)
}

// RowText returns text, the text of a row of the parse whose record is
// parse, as the run shows it: as Text does, and with the parse's shorter
// secret values and its readers' details replaced too.
func (r *Recorder) RowText(text string, parse *Found) string {
	return r.replace(text, parse, rowText, parseProblem)
}

// ProblemText returns text, a problem line of the parse whose record is
// parse (nil for a line of no parse), as the run shows it: as Text does,
// and with the parse's readers' details replaced too.
func (r *Recorder) ProblemText(text string, parse *Found) string {
	return r.replace(text, parse, parseProblem)
}

// replace returns text with the credentials of its URLs, the secrets of
// everywhere and those parse recorded in each of reaches replaced. Secrets
// that overlap are one replaced range (replaceRanges), so a secret inside a
// longer one (a password inside user:password) goes with it. The output is
// built from the original and the placeholder, never read again. The cost
// is the text's length once for the URLs and once per record that holds a
// text to replace, after each record's matcher is built once.
func (r *Recorder) replace(text string, parse *Found, reaches ...reach) string {
	if r == nil || text == "" {
		return text
	}
	emptyPlaceholder := r.setting.Placeholder == ""
	var matchers []*textset.Set
	if len(r.everywhere.texts[everyString]) > 0 {
		matchers = append(matchers, r.everywhere.matcherOf(everyString, emptyPlaceholder, r.work))
	}
	for _, where := range reaches {
		if parse != nil && len(parse.texts[where]) > 0 {
			matchers = append(matchers, parse.matcherOf(where, emptyPlaceholder, r.work))
		}
	}
	var inText []urlcredentials.Credential
	identifyURLs(text, func(secret urlcredentials.Credential) { inText = append(inText, secret) }, r.work)
	if len(matchers) == 0 && len(inText) == 0 {
		return text
	}
	longest := make([]int, len(text))
	for _, matcher := range matchers {
		matcher.Longest(text, longest)
	}
	// The credentials of a URL are replaced where the URL holds them,
	// whatever their length, by their byte range: the same letters
	// elsewhere in the string are not a credential.
	for _, secret := range inText {
		longest[secret.Start] = max(longest[secret.Start], secret.ReplacedEnd(emptyPlaceholder)-secret.Start)
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

// matcherOf returns the matcher of the record's texts replaced in where
// (the forms with a part only for an empty placeholder), building it again
// only when texts were recorded since.
func (f *Found) matcherOf(where reach, withParts bool, work textset.Counter) *textset.Set {
	texts := f.texts[where]
	if f.matchers[where] == nil || f.matched[where] != len(texts) {
		var matched []string
		for _, text := range texts {
			if withParts || f.forms[placed{text, where}].plain() {
				matched = append(matched, text)
			}
		}
		f.matchers[where], f.matched[where] = textset.NewBackward(matched, work), len(texts)
	}
	return f.matchers[where]
}
