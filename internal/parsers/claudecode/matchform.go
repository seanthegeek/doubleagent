package claudecode

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/dedup"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The names under which the match forms keep what they learn of a shared
// text: the prompt form's segments, whether a text holds a '>', and where
// the typed-command test stands after a text.
const (
	promptFormName   = "claude-code prompt"
	closingMemoName  = "claude-code closing bracket"
	commandMemoName  = "claude-code typed command"
	wrapperName      = "pasted_content"
	formOutputLength = 4096
)

// The states of the paste wrapper test of promptForm: nothing held back,
// "<" or "</" held back, one of them with the first bytes of wrapperName
// (wrapperNamed), and inside a tag with attributes, whose bytes are
// dropped up to its '>'.
const (
	wrapperNone uint8 = iota
	wrapperLess
	wrapperSlash
	wrapperNamed
	wrapperInTag = wrapperNamed + 2*uint8(len(wrapperName))
)

// The states of the white space collapse of promptForm: nothing written
// yet, a byte that is not white space written last, and white space seen
// after one, written as one space before the next such byte.
const (
	spaceStart = iota
	spaceAfterText
	spacePending
)

// promptForm rewrites a prompt into its match form a piece at a time:
// each paste wrapper tag (<pasted_content ...> or </pasted_content>)
// becomes one space, then each run of ASCII white space becomes one space
// and the spaces at both ends go. A tag with attributes ends at the first
// '>' after it and is a tag only when one follows; the caller says whether
// a later piece holds one. A form's state is a few small numbers, so what a
// shared text becomes is computed once per state (dedup.MatchText.Segment).
type promptForm struct {
	wrapper, space uint8
}

// formOutput collects what a promptForm writes and hands it to yield in
// pieces of up to formOutputLength bytes, noting when yield asks to stop.
type formOutput struct {
	held    []byte
	yield   func(string) bool
	stopped bool
}

// add writes one byte.
func (o *formOutput) add(character byte) {
	o.held = append(o.held, character)
	if len(o.held) >= formOutputLength {
		o.flush()
	}
}

// flush hands what is held to yield.
func (o *formOutput) flush() {
	if len(o.held) > 0 && !o.stopped {
		o.stopped = !o.yield(string(o.held))
	}
	o.held = o.held[:0]
}

// state returns the form's state as one number.
func (f promptForm) state() uint64 {
	return uint64(f.wrapper)<<2 | uint64(f.space)
}

// formAt returns the form of a state number.
func formAt(state uint64) promptForm {
	return promptForm{wrapper: uint8(state >> 2), space: uint8(state & 3)}
}

// feed rewrites text, closingLater saying whether a later piece holds a
// '>'; it stops early when out has stopped.
func (f *promptForm) feed(text string, closingLater bool, out *formOutput) {
	lastClosing := strings.LastIndexByte(text, '>')
	for i := 0; i < len(text) && !out.stopped; i++ {
		f.step(text[i], closingLater || lastClosing > i, out)
	}
}

// step rewrites one byte, closingAfter saying whether a '>' follows it.
func (f *promptForm) step(character byte, closingAfter bool, out *formOutput) {
	switch f.wrapper {
	case wrapperInTag:
		if character == '>' {
			f.wrapper = wrapperNone
		}
		return
	case wrapperNone:
		if character == '<' {
			f.wrapper = wrapperLess
		} else {
			f.put(character, out)
		}
		return
	case wrapperLess, wrapperSlash:
		switch {
		case character == '/' && f.wrapper == wrapperLess:
			f.wrapper = wrapperSlash
			return
		case character == wrapperName[0]:
			f.wrapper = named(f.wrapper == wrapperSlash, 1)
			return
		}
	default:
		if f.stepNamed(character, closingAfter, out) {
			return
		}
	}
	f.release(out)
	f.step(character, closingAfter, out)
}

// named returns the state holding back "<", or "</" when closing, and the
// first length bytes of wrapperName.
func named(closing bool, length int) uint8 {
	if closing {
		length += len(wrapperName)
	}
	return uint8(int(wrapperNamed) + length - 1)
}

// heldName returns whether the state holds back "</" rather than "<", and
// how many bytes of wrapperName it holds back.
func (f promptForm) heldName() (closing bool, length int) {
	offset := int(f.wrapper) - int(wrapperNamed)
	return offset >= len(wrapperName), offset%len(wrapperName) + 1
}

// stepNamed takes one byte in a state holding back part of a tag name,
// reporting false when the byte shows that what is held back is no tag.
func (f *promptForm) stepNamed(character byte, closingAfter bool, out *formOutput) bool {
	closing, length := f.heldName()
	switch {
	case length < len(wrapperName) && character == wrapperName[length]:
		f.wrapper = named(closing, length+1)
	case length == len(wrapperName) && character == '>':
		f.wrapper = wrapperNone
		f.put(' ', out)
	case length == len(wrapperName) && isTagSpace(character) && closingAfter:
		f.wrapper = wrapperInTag
		f.put(' ', out)
	default:
		return false
	}
	return true
}

// isTagSpace reports whether a byte may follow wrapperName in a tag with
// attributes.
func isTagSpace(character byte) bool {
	return character == ' ' || character == '\t' || character == '\r' || character == '\n'
}

// release writes the bytes of a wrapper tag held back, which are not one.
func (f *promptForm) release(out *formOutput) {
	held := *f
	f.wrapper = wrapperNone
	switch held.wrapper {
	case wrapperNone, wrapperInTag:
		return
	case wrapperLess:
		f.put('<', out)
		return
	case wrapperSlash:
		f.put('<', out)
		f.put('/', out)
		return
	}
	closing, length := held.heldName()
	f.put('<', out)
	if closing {
		f.put('/', out)
	}
	for i := range length {
		f.put(wrapperName[i], out)
	}
}

// put writes one byte through the white space collapse.
func (f *promptForm) put(character byte, out *formOutput) {
	switch {
	case isASCIISpace(character):
		if f.space == spaceAfterText {
			f.space = spacePending
		}
		return
	case f.space == spacePending:
		out.add(' ')
	}
	out.add(character)
	f.space = spaceAfterText
}

// isASCIISpace reports whether a byte is ASCII white space.
func isASCIISpace(character byte) bool {
	return character == ' ' || character == '\t' || character == '\n' || character == '\r' ||
		character == '\f' || character == '\v'
}

// promptPiece is a piece of a prompt's text: literal text, or a reference
// to a shared text; closingAfter says whether a later piece holds a '>'.
type promptPiece struct {
	literal      string
	ref          *timeline.SharedRef
	closingAfter bool
}

// promptPieces returns the pieces of a row's text as it reads in full, with
// whether a later piece holds a '>', which whether a tag with attributes
// is one depends on.
func promptPieces(row timeline.Row, text *dedup.MatchText) []promptPiece {
	var pieces []promptPiece
	last := 0
	if row.Shared != nil {
		for i := range *row.Shared {
			ref := &(*row.Shared)[i]
			pieces = append(pieces, promptPiece{literal: row.Text[last:ref.Start]},
				promptPiece{ref: ref})
			last = ref.End
		}
	}
	pieces = append(pieces, promptPiece{literal: row.Text[last:]})
	closing := false
	for i := len(pieces) - 1; i >= 0; i-- {
		pieces[i].closingAfter = closing
		if pieces[i].ref == nil {
			closing = closing || strings.IndexByte(pieces[i].literal, '>') >= 0
		} else {
			closing = closing || text.Memo(*pieces[i].ref, closingMemoName, 0, holdsClosing) == 1
		}
	}
	return pieces
}

// holdsClosing returns 1 when text holds a '>', else 0.
func holdsClosing(text string) uint64 {
	if strings.IndexByte(text, '>') >= 0 {
		return 1
	}
	return 0
}

// writePrompt writes the match form of a prompt row to text, each shared
// text it names as a segment of the form's state where it begins.
func writePrompt(row timeline.Row, text *dedup.MatchText) {
	form := promptForm{wrapper: wrapperNone, space: spaceStart}
	var literal strings.Builder
	out := &formOutput{yield: func(piece string) bool {
		literal.WriteString(piece)
		return true
	}}
	for _, piece := range promptPieces(row, text) {
		if piece.ref == nil {
			form.feed(piece.literal, piece.closingAfter, out)
			continue
		}
		out.flush()
		text.Write(literal.String())
		literal.Reset()
		state := form.state()<<1 | bitOf(piece.closingAfter)
		form = formAt(text.Segment(*piece.ref, promptFormName, state, writeSegment))
	}
	form.release(out)
	out.flush()
	text.Write(literal.String())
}

// bitOf returns 1 for true and 0 for false.
func bitOf(value bool) uint64 {
	if value {
		return 1
	}
	return 0
}

// writeSegment writes the match form of a shared text from the state a
// segment names (the form's state and whether a later piece holds a '>')
// and returns the form's state at its end.
func writeSegment(full string, state uint64, yield func(string) bool) uint64 {
	form := formAt(state >> 1)
	out := &formOutput{yield: yield}
	form.feed(full, state&1 == 1, out)
	out.flush()
	return form.state()
}

// The states of typedCommand's test: at the start, after the slash, in
// the command's name, and decided either way.
const (
	commandStart = iota
	commandSlash
	commandName
	commandTyped
	commandNot
)

// typedCommand reports whether a row's text as it reads in full is a slash
// command as typed (/model, /rename x): a slash, an ASCII letter, then
// letters, digits, ':', '_' and '-' up to a space, a tab or the end. Where
// a shared text is reached, the test's state after it is kept in texts.
func typedCommand(row timeline.Row, texts *dedup.Texts) bool {
	state := uint64(commandStart)
	last := 0
	if row.Shared != nil {
		for _, ref := range *row.Shared {
			state = commandAfter(row.Text[last:ref.Start], state)
			if state >= commandTyped {
				break
			}
			state = texts.Memo(ref, commandMemoName, state, func(full string) uint64 {
				return commandAfter(full, state)
			})
			last = ref.End
		}
	}
	if state < commandTyped {
		state = commandAfter(row.Text[last:], state)
	}
	return state == commandTyped || state == commandName
}

// commandAfter returns typedCommand's state after text, from state.
func commandAfter(text string, state uint64) uint64 {
	for i := 0; i < len(text) && state < commandTyped; i++ {
		character := text[i]
		switch {
		case state == commandStart && character == '/':
			state = commandSlash
		case state == commandSlash && isASCIILetter(character):
			state = commandName
		case state == commandName && (isASCIILetter(character) || '0' <= character && character <= '9' ||
			character == ':' || character == '_' || character == '-'):
		case state == commandName && (character == ' ' || character == '\t'):
			state = commandTyped
		default:
			state = commandNot
		}
	}
	return state
}

// isASCIILetter reports whether a byte is an ASCII letter.
func isASCIILetter(character byte) bool {
	return 'a' <= character && character <= 'z' || 'A' <= character && character <= 'Z'
}
