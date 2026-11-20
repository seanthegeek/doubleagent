package dedup

import (
	"crypto/rand"
	"encoding/binary"
	"math/bits"
)

// modulus is the prime 2^61 - 1 the text digests are taken modulo: a
// product of two values below it fits the 128 bits bits.Mul64 gives, and a
// value modulo it is reduced with a shift and an add.
const modulus = 1<<61 - 1

// digest is the length of a text and its polynomial hash: the sum of each
// byte times base to the power of the number of bytes after it, modulo
// modulus. The digest of two texts joined is computed from theirs alone
// (hasher.join), so a text many rows name is read once for all of them.
// Two different texts of one length share a digest for at most length - 1
// of the possible bases, since their difference is a nonzero polynomial of
// that degree; with the base drawn at random for each run, that chance is
// below length / 2^61 for each pair compared. A digest only finds the rows
// to compare: classOf compares their texts byte for byte, so two different
// texts with one digest are never taken for one another.
type digest struct {
	length int
	hash   uint64
}

// hasher computes digests with one base.
type hasher struct {
	base uint64
}

// randomBase returns a base drawn at random from 2 to modulus - 2, so
// that no one can write two texts that share a digest before the run that
// compares them.
func randomBase() uint64 {
	var random [8]byte
	_, _ = rand.Read(random[:])
	return 2 + binary.LittleEndian.Uint64(random[:])%(modulus-3)
}

// mulMod returns left times right modulo modulus, for values below it.
func mulMod(left, right uint64) uint64 {
	high, low := bits.Mul64(left, right)
	// The product is below 2^122: its bits from 61 up are worth their
	// value again, since 2^61 is 1 modulo the modulus.
	sum := low&modulus + (high<<3 | low>>61)
	if sum >= modulus {
		sum -= modulus
	}
	return sum
}

// powMod returns the base to the power of exponent modulo modulus.
func (h hasher) powMod(exponent int) uint64 {
	result, square := uint64(1), h.base
	for ; exponent > 0; exponent >>= 1 {
		if exponent&1 == 1 {
			result = mulMod(result, square)
		}
		square = mulMod(square, square)
	}
	return result
}

// extend returns the digest of the text of start followed by text.
func (h hasher) extend(start digest, text string) digest {
	hash := start.hash
	for i := range len(text) {
		hash = mulMod(hash, h.base) + uint64(text[i])
		if hash >= modulus {
			hash -= modulus
		}
	}
	return digest{start.length + len(text), hash}
}

// join returns the digest of the text of left followed by that of right.
func (h hasher) join(left, right digest) digest {
	hash := mulMod(left.hash, h.powMod(right.length)) + right.hash
	if hash >= modulus {
		hash -= modulus
	}
	return digest{left.length + right.length, hash}
}
