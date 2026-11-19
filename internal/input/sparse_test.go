package input

import (
	"archive/tar"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// sparseRefusal is the text of the refusal of a sparse tar member.
const sparseRefusal = "sparse members are not supported"

// sparseMemberName is the name of the member the sparse archives hold.
const sparseMemberName = "home/alice/.claude/history.jsonl"

// tarBlock is the size of a tar header block and of the padding of a body.
const tarBlock = 512

// headerBlock returns one tar header block for a member with the given type
// letter, name, body size and magic field (eight bytes), with its checksum.
func headerBlock(typeflag byte, name string, size int, magic string) []byte {
	block := make([]byte, tarBlock)
	copy(block[0:], name)
	copy(block[100:], "0000644\x00")
	copy(block[124:], fmt.Sprintf("%011o\x00", size))
	block[156] = typeflag
	copy(block[257:], magic)
	for i := 148; i < 156; i++ {
		block[i] = ' '
	}
	sum := 0
	for _, b := range block {
		sum += int(b)
	}
	copy(block[148:], fmt.Sprintf("%06o\x00 ", sum))
	return block
}

// paddedBody returns body followed by zero bytes up to a whole block.
func paddedBody(body []byte) []byte {
	return append(body, make([]byte, (tarBlock-len(body)%tarBlock)%tarBlock)...)
}

// paxRecord returns one extended header record: its length, the key, the
// value and a newline, the length counting its own digits.
func paxRecord(key, value string) string {
	rest := " " + key + "=" + value + "\n"
	length := len(rest) + 1
	for len(strconv.Itoa(length))+len(rest) != length {
		length = len(strconv.Itoa(length)) + len(rest)
	}
	return strconv.Itoa(length) + rest
}

// endOfTar is the two empty blocks that end an archive.
func endOfTar() []byte { return make([]byte, 2*tarBlock) }

// oldGNUSparseTar returns an archive whose one member has the type letter
// of the old GNU sparse form.
func oldGNUSparseTar() []byte {
	archive := headerBlock(tar.TypeGNUSparse, sparseMemberName, 0, "ustar  \x00")
	return append(archive, endOfTar()...)
}

// paxSparseTar returns an archive whose one member has the extended header
// records of the PAX sparse form, a map of no data blocks.
func paxSparseTar() []byte {
	var records strings.Builder
	for _, record := range [][2]string{{"GNU.sparse.major", "0"}, {"GNU.sparse.minor", "1"},
		{"GNU.sparse.numblocks", "0"}, {"GNU.sparse.map", ""}, {"GNU.sparse.realsize", "0"}} {
		records.WriteString(paxRecord(record[0], record[1]))
	}
	archive := headerBlock(tar.TypeXHeader, "PaxHeaders/sparse", records.Len(), "ustar\x0000")
	archive = append(archive, paddedBody([]byte(records.String()))...)
	archive = append(archive, headerBlock(tar.TypeReg, sparseMemberName, 0, "ustar\x0000")...)
	return append(archive, endOfTar()...)
}

func TestASparseTarMemberOfEitherFormIsRefusedAsSparse(t *testing.T) {
	for name, data := range map[string][]byte{
		"the old GNU form": oldGNUSparseTar(),
		"the PAX form":     paxSparseTar(),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := openArchiveOf(t, "sparse.tar", data)
			if err == nil || !strings.Contains(err.Error(), sparseRefusal) {
				t.Errorf("error %v, want one holding %q", err, sparseRefusal)
			}
		})
	}
}
