package parserregistry

import (
	"bytes"
	"maps"
	"slices"
	"sync/atomic"
	"testing"
)

// The planted-repeat test holds every parser to the kit's rule that a
// repeat is reported once, by the parse of the file that holds it when a
// parser takes that file as its own artifact, else by the one parse that
// reads it: each file of each growth case is written with one object's
// member renamed to the name of an earlier member of the same object, the
// files around it as they were, and the repeat rows may grow by one at
// most, on that file when it is parsed as its own artifact.
// The rename keeps every byte count, so it reaches JSON inside a SQLite
// cell and inside a zstd frame of raw blocks as well as a text file.

// plantRepeat returns data with the first object found whose members hold
// two different names of one length given a repeat, or with every such
// object given one when every is set (each record of a file, each row of a
// database): the later name is overwritten with the earlier. It reports
// false when data holds no such object.
func plantRepeat(data []byte, every bool) ([]byte, bool) {
	planted, found := bytes.Clone(data), false
	for start := bytes.IndexByte(planted, '{'); start >= 0; {
		if later, earlier, ok := equalLengthNames(planted, start); ok {
			copy(planted[later:], planted[earlier:earlier+nameLength(planted, earlier)])
			if found = true; !every {
				break
			}
		}
		next := bytes.IndexByte(planted[start+1:], '{')
		if next < 0 {
			break
		}
		start += 1 + next
	}
	return planted, found
}

// nameLength returns the length of the JSON string that starts at offset,
// its quotes included, or 0 when none ends there.
func nameLength(data []byte, offset int) int {
	for i := offset + 1; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++
		case '"':
			return i + 1 - offset
		}
	}
	return 0
}

// equalLengthNames reads the object that starts at start and returns the
// offsets of a member name and of an earlier, different name of the same
// length, or false when the object is not well formed or has none.
func equalLengthNames(data []byte, start int) (int, int, bool) {
	seen := map[int]int{}
	i := skipSpace(data, start+1)
	for i < len(data) && data[i] == '"' {
		length := nameLength(data, i)
		if length == 0 {
			return 0, 0, false
		}
		if earlier, ok := seen[length]; ok && !bytes.Equal(data[earlier:earlier+length], data[i:i+length]) {
			return i, earlier, true
		}
		if _, ok := seen[length]; !ok {
			seen[length] = i
		}
		i = skipSpace(data, i+length)
		if i >= len(data) || data[i] != ':' {
			return 0, 0, false
		}
		end, ok := skipValue(data, skipSpace(data, i+1))
		if !ok {
			return 0, 0, false
		}
		i = skipSpace(data, end)
		if i < len(data) && data[i] == ',' {
			i = skipSpace(data, i+1)
		}
	}
	return 0, 0, false
}

// skipSpace returns the first offset from i that is not JSON whitespace.
func skipSpace(data []byte, i int) int {
	for i < len(data) && bytes.IndexByte([]byte(" \t\r\n"), data[i]) >= 0 {
		i++
	}
	return i
}

// skipValue returns the offset after the JSON value that starts at i: a
// string, or anything up to the comma or bracket that closes it at its own
// depth, strings skipped; false when the data ends first.
func skipValue(data []byte, i int) (int, bool) {
	depth := 0
	for ; i < len(data); i++ {
		switch data[i] {
		case '"':
			length := nameLength(data, i)
			if length == 0 {
				return 0, false
			}
			i += length - 1
			if depth == 0 {
				return i + 1, true
			}
		case '{', '[':
			depth++
		case '}', ']':
			if depth == 0 {
				return i, true
			}
			if depth--; depth == 0 {
				return i + 1, true
			}
		case ',':
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

// repeatRowsByFile returns the repeat rows of a parse of the files by
// source file, and the source files a parser takes as its own artifact.
func repeatRowsByFile(t *testing.T, name string, test growthCase, files []growthFile) (map[string]int,
	map[string]bool) {
	t.Helper()
	return repeatRowsAndOwners(t, name, test.agent, files)
}

// plantedOutcome is what planting a repeat in each file of a case gave:
// the files planted and those whose parse gave its row.
type plantedOutcome struct {
	planted, reported int
}

// checkPlanted plants a repeat in one object, then in every object, of
// each file of the case in turn and checks that the repeat rows grow by one
// at most, only on the planted file when it is parsed as its own artifact.
// The files as written are parsed once, for both kinds of planting.
func checkPlanted(t *testing.T, name string, test growthCase) plantedOutcome {
	files := test.files(t, 4)
	before, own := repeatRowsByFile(t, name, test, files)
	var outcome plantedOutcome
	for _, every := range []bool{false, true} {
		found := checkPlantedIn(t, name, test, plantedBase{files, before, own}, every)
		outcome.planted += found.planted
		outcome.reported += found.reported
	}
	return outcome
}

// plantedBase is a case's files as written, with the repeat rows their
// parse gave by source file and the source files parsed as their own
// artifact.
type plantedBase struct {
	files  []growthFile
	before map[string]int
	own    map[string]bool
}

// checkPlantedIn holds the case to checkPlanted's rule with a repeat in
// the first object of the planted file, or in every object when every is
// set.
func checkPlantedIn(t *testing.T, name string, test growthCase, base plantedBase, every bool) plantedOutcome {
	files, before, own := base.files, base.before, base.own
	var outcome plantedOutcome
	for k, file := range files {
		data, ok := plantRepeat(file.data, every)
		if !ok {
			continue
		}
		outcome.planted++
		planted := slices.Clone(files)
		planted[k] = growthFile{rel: file.rel, data: data}
		after, _ := repeatRowsByFile(t, name, test, planted)
		source := "/home/u/" + file.rel
		total := 0
		for _, key := range slices.Sorted(maps.Keys(after)) {
			grew := after[key] - before[key]
			total += grew
			if own[source] && key != source && grew != 0 {
				t.Errorf("a repeat planted in %s, parsed as its own artifact, gave %d more repeat rows on %s",
					file.rel, grew, key)
			}
		}
		if total > 1 {
			t.Errorf("a repeat planted in %s gave %d more repeat rows", file.rel, total)
		}
		if after[source] > before[source] {
			outcome.reported++
		}
	}
	return outcome
}

func TestARepeatIsReportedOnceByTheParseOfTheFileThatHoldsIt(t *testing.T) {
	var planted, reported atomic.Int64
	// The cases run in parallel, so the totals are logged once they are all
	// done: a test's cleanup runs after its subtests.
	t.Cleanup(func() {
		t.Logf("%d files planted, %d reported by their own parse", planted.Load(), reported.Load())
	})
	for _, set := range growthSets() {
		for _, test := range set.cases {
			if test.shape == shapeUnlisted {
				continue
			}
			t.Run(set.parser+"/"+test.kind+"/"+test.shape, func(t *testing.T) {
				t.Parallel()
				outcome := checkPlanted(t, set.parser, test)
				planted.Add(int64(outcome.planted))
				reported.Add(int64(outcome.reported))
			})
		}
	}
}
