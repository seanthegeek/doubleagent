package dedup

// textClass is one class of equal match texts: its number, a member that
// names no shared text (plain), the first member, and the arrangements of
// its members that name some (MatchText.arrangement), each of which is
// then known to be equal without reading it again.
type textClass struct {
	number       int
	plain, first *MatchText
	arrangements map[string]bool
}

// classOf returns the number of the class of a match text, comparing it
// with the classes of the same digest. A text whose arrangement a class
// already holds joins it unread; any other is compared byte for byte, with
// the class's plain member when it has one, so a shared text is read for a
// comparison only once per arrangement that holds it. Two texts with one
// digest that differ are never put in one class.
func (x *Texts) classOf(text *MatchText) int {
	arrangement := ""
	if text.segmented {
		arrangement = text.arrangement()
	}
	for _, class := range x.groups[text.digest] {
		if class.holds(text, arrangement) {
			if text.segmented {
				class.arrangements[arrangement] = true
			} else if class.plain == nil {
				class.plain = text
			}
			return class.number
		}
	}
	x.classes++
	class := &textClass{number: x.classes, first: text, arrangements: map[string]bool{}}
	if text.segmented {
		class.arrangements[arrangement] = true
	} else {
		class.plain = text
	}
	x.groups[text.digest] = append(x.groups[text.digest], class)
	return class.number
}

// holds reports whether a match text with an arrangement ("" when it names
// no shared text) is equal to the members of the class.
func (c *textClass) holds(text *MatchText, arrangement string) bool {
	if text.segmented && c.arrangements[arrangement] {
		return true
	}
	member := c.plain
	if member == nil {
		member = c.first
	}
	return sameText(text, member)
}

// sameText reports whether two match texts are equal byte for byte,
// reading them only as far as their first difference. The left text is
// read as its writers write it and the right through a pieceCursor, so the
// comparison needs no coroutine. A text naming no shared text is read by
// the cursor, which then runs no writer and holds no piece; when both name
// one, the cursor writes a segment of the right text whole before reading
// it.
func sameText(left, right *MatchText) bool {
	if right.segmented && !left.segmented {
		left, right = right, left
	}
	rest := pieceCursor{text: right}
	for _, part := range left.parts {
		if part.segment == nil {
			if !rest.consume(part.literal) {
				return false
			}
		} else if !part.each(left.texts, rest.consume) {
			return false
		}
	}
	// Equal only when nothing of the right text is left: neither the rest
	// of the piece the cursor is at nor a later piece.
	return rest.piece == "" && !rest.next()
}

// pieceCursor steps through the pieces of a match text: the part it reads
// next, the pieces of the segment it is in (its writer's output, held
// until read, the only pieces it holds) and the unread rest of the piece
// it is at.
type pieceCursor struct {
	text     *MatchText
	part     int
	buffered []string
	piece    string
}

// next moves the cursor to the next piece that is not empty and reports
// whether there was one.
func (c *pieceCursor) next() bool {
	for {
		if len(c.buffered) > 0 {
			c.piece, c.buffered = c.buffered[0], c.buffered[1:]
		} else if c.part < len(c.text.parts) {
			c.part++
			c.readPart(c.text.parts[c.part-1])
		} else {
			return false
		}
		if c.piece != "" {
			return true
		}
	}
}

// readPart puts the cursor at the start of a part: at a literal part's
// text, or before the pieces of a segment, written again in full.
func (c *pieceCursor) readPart(part textPart) {
	c.piece = ""
	if part.segment == nil {
		c.piece = part.literal
		return
	}
	c.buffered = c.buffered[:0]
	part.each(c.text.texts, func(chunk string) bool {
		c.buffered = append(c.buffered, chunk)
		return true
	})
}

// consume reports whether the text at the cursor begins with chunk, and
// moves the cursor past it when it does.
func (c *pieceCursor) consume(chunk string) bool {
	for chunk != "" {
		if c.piece == "" && !c.next() {
			return false
		}
		n := min(len(chunk), len(c.piece))
		if chunk[:n] != c.piece[:n] {
			return false
		}
		chunk, c.piece = chunk[n:], c.piece[n:]
	}
	return true
}
