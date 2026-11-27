package secrets

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/secrets/urlcredentials"
	"github.com/seanthegeek/doubleagent/internal/textset"
)

// matchedTexts appends to texts the record's texts replaced in where that
// a matcher matches: every one with withParts, else the plain forms only.
func (f *Found) matchedTexts(texts []string, where reach, withParts bool) []string {
	for _, text := range f.texts[where] {
		if withParts || f.forms[placed{text, where}].plain() {
			texts = append(texts, text)
		}
	}
	return texts
}

// matcherOf returns the matcher of the texts records hold in reaches,
// building it again only when the records, the reaches or what they hold
// differ from the last call's; a row usually lists the same records as the
// row before.
func (j *joinedMatcher) matcherOf(records []*Found, reaches []reach, withParts bool,
	work textset.Counter) *textset.Set {
	changes := 0
	for _, record := range records {
		for _, where := range reaches {
			changes += record.changes[where]
		}
	}
	if j.set != nil && changes == j.changes && withParts == j.withParts && slices.Equal(records, j.records) &&
		slices.Equal(reaches, j.reaches) {
		return j.set
	}
	var texts []string
	for _, record := range records {
		for _, where := range reaches {
			texts = record.matchedTexts(texts, where, withParts)
		}
	}
	*j = joinedMatcher{slices.Clone(records), slices.Clone(reaches), withParts, changes,
		textset.NewBackward(texts, work)}
	return j.set
}

// matcherOf returns the matcher of the record's texts replaced in where
// (the forms with a part only for an empty placeholder), building it again
// only when a text was recorded or a form made plain since.
func (f *Found) matcherOf(where reach, withParts bool, work textset.Counter) *textset.Set {
	if f.matchers[where] == nil || f.matched[where] != f.changes[where] {
		f.matchers[where] = textset.NewBackward(f.matchedTexts(nil, where, withParts), work)
		f.matched[where] = f.changes[where]
	}
	return f.matchers[where]
}

// appendMatchers appends to matchers the record's matcher of each of
// reaches that holds a text.
func (f *Found) appendMatchers(matchers []*textset.Set, reaches []reach, withParts bool,
	work textset.Counter) []*textset.Set {
	for _, where := range reaches {
		if len(f.texts[where]) > 0 {
			matchers = append(matchers, f.matcherOf(where, withParts, work))
		}
	}
	return matchers
}

// parseMatchers appends to matchers those that search a text of length
// bytes for the secrets each record parse lists holds in reaches. A lone
// record, or a listed one holding more bytes there than the text, is
// searched with its own matchers, each built once in the run; the other
// listed records share one matcher of all their texts. So a row costs its
// length a fixed number of times, plus, for each listed record, the smaller
// of its bytes and the row's length, however many records it lists.
func (r *Recorder) parseMatchers(matchers []*textset.Set, length int, parse *Found, reaches []reach,
	withParts bool) []*textset.Set {
	records := parse.Records()
	var joined []*Found
	for _, record := range records {
		size := 0
		for _, where := range reaches {
			size += record.size[where]
		}
		if size > 0 && len(records) > 1 && size <= length {
			joined = append(joined, record)
			continue
		}
		matchers = record.appendMatchers(matchers, reaches, withParts, r.work)
	}
	if len(joined) == 1 {
		return joined[0].appendMatchers(matchers, reaches, withParts, r.work)
	}
	if len(joined) > 1 {
		matchers = append(matchers, r.joined.matcherOf(joined, reaches, withParts, r.work))
	}
	return matchers
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

// joinedMatcher is one matcher of the texts several records hold in some
// reaches, with what it was built from: the records, the reaches, whether
// forms with a part count, and the sum of the records' changes there then,
// which grows with any change, since each record's count only grows.
type joinedMatcher struct {
	records   []*Found
	reaches   []reach
	withParts bool
	changes   int
	set       *textset.Set
}

// replace returns text with the credentials of its URLs, the secrets of
// everywhere and those each parse parse stands for (Records) recorded in
// each of reaches replaced. Secrets that overlap are one replaced range
// (replaceRanges), so a secret inside a longer one (a password inside
// user:password) goes with it. The output is built from the original and
// the placeholder, never read again. The cost is the text's length a fixed
// number of times, plus what parseMatchers indexes.
func (r *Recorder) replace(text string, parse *Found, reaches ...reach) string {
	if r == nil || text == "" {
		return text
	}
	emptyPlaceholder := r.setting.Placeholder == ""
	var matchers []*textset.Set
	if len(r.everywhere.texts[everyString]) > 0 {
		matchers = append(matchers, r.everywhere.matcherOf(everyString, emptyPlaceholder, r.work))
	}
	matchers = r.parseMatchers(matchers, len(text), parse, reaches, emptyPlaceholder)
	var inText []urlcredentials.Credential
	identifyURLs(text, func(secret urlcredentials.Credential) { inText = append(inText, secret) }, r.work)
	tails := r.cutTails(emptyPlaceholder)
	if len(matchers) == 0 && len(inText) == 0 && tails.set == nil {
		return text
	}
	longest := make([]int, len(text))
	for _, matcher := range matchers {
		matcher.Longest(text, longest)
	}
	tails.withhold(text, longest)
	// The credentials of a URL are replaced where the URL holds them,
	// whatever their length, by their byte range: the same letters
	// elsewhere in the string are not a credential.
	for _, secret := range inText {
		longest[secret.Start] = max(longest[secret.Start], secret.ReplacedEnd(emptyPlaceholder)-secret.Start)
	}
	return replaceRanges(text, longest, r.setting.Placeholder)
}

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
// secret values and its readers' details replaced too; for a row kept for
// dropped duplicates (WithDuplicates), those of each parse it stands for.
func (r *Recorder) RowText(text string, parse *Found) string {
	return r.replace(text, parse, rowText, parseProblem)
}

// ProblemText returns text, a problem line of the parse whose record is
// parse (nil for a line of no parse), as the run shows it: as Text does,
// and with the parse's readers' details replaced too.
func (r *Recorder) ProblemText(text string, parse *Found) string {
	return r.replace(text, parse, parseProblem)
}
