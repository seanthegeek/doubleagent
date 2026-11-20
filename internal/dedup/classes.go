package dedup

import "iter"

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
	return sameText(text.chunks(), member.chunks())
}

// sameText reports whether two texts given in pieces are equal, reading
// them only as far as their first difference.
func sameText(left, right iter.Seq[string]) bool {
	nextLeft, stopLeft := iter.Pull(left)
	defer stopLeft()
	nextRight, stopRight := iter.Pull(right)
	defer stopRight()
	var leftChunk, rightChunk string
	for {
		leftMore, rightMore := true, true
		for leftChunk == "" && leftMore {
			leftChunk, leftMore = nextLeft()
		}
		for rightChunk == "" && rightMore {
			rightChunk, rightMore = nextRight()
		}
		if !leftMore || !rightMore {
			return leftMore == rightMore
		}
		n := min(len(leftChunk), len(rightChunk))
		if leftChunk[:n] != rightChunk[:n] {
			return false
		}
		leftChunk, rightChunk = leftChunk[n:], rightChunk[n:]
	}
}
