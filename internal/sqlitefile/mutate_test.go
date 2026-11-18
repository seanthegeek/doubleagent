package sqlitefile

import (
	"bytes"
	"encoding/binary"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// fixtureSet is one checked-in database with the sidecars beside it; a nil
// log or journal is absent.
type fixtureSet struct {
	name               string
	main, log, journal []byte
}

// fixtureSets returns every database of testdata and of its fixture
// archive, with its -wal and -journal files.
func fixtureSets(t *testing.T) []fixtureSet {
	t.Helper()
	files := archivedFiles(t)
	entries, err := osfile.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			files[entry.Name()] = testdataBytes(t, entry.Name())
		}
	}
	var sets []fixtureSet
	for name, data := range files {
		if bytes.HasPrefix(data, []byte(headerString)) || strings.HasSuffix(name, ".db") && len(data) == 0 {
			sets = append(sets, fixtureSet{name: name, main: data, log: files[name+"-wal"],
				journal: files[name+"-journal"]})
		}
	}
	slices.SortFunc(sets, func(left, right fixtureSet) int { return strings.Compare(left.name, right.name) })
	return sets
}

// mutate changes a few bytes of data, near page headers or anywhere, and
// now and then cuts it short, as damage does.
func mutate(random *rand.Rand, data []byte) []byte {
	data = bytes.Clone(data)
	if len(data) == 0 {
		return data
	}
	for range 1 + random.IntN(4) {
		place := random.IntN(len(data))
		if random.IntN(2) == 0 {
			pageSize := 512 << random.IntN(4)
			place = min(place/pageSize*pageSize+random.IntN(16), len(data)-1)
		}
		switch random.IntN(4) {
		case 0:
			data[place] ^= 1 << random.IntN(8)
		case 1:
			data[place] = byte(random.IntN(256))
		case 2:
			data[place] = 0xff
		default:
			if place+4 <= len(data) {
				binary.BigEndian.PutUint32(data[place:], uint32(random.IntN(40)))
			}
		}
	}
	if random.IntN(6) == 0 {
		data = data[:random.IntN(len(data)+1)]
	}
	return data
}

// readMeasured reads everything a database offers, walk by walk, and
// returns the most bytes one walk read from its files, the pages times
// their size that bound it, and the bytes the whole read took from them.
func readMeasured(main, log, journal []byte) (int64, int64, int64) {
	var read int64
	database, err := Open(countedFiles(&read, main, log, journal))
	if err != nil {
		return 0, 0, read
	}
	most := int64(0)
	measure := func(walk func()) {
		before := read
		walk()
		most = max(most, read-before)
	}
	var schema []SchemaRow
	measure(func() { schema, _ = database.Schema() })
	for _, row := range schema {
		for _, rows := range []*Rows{database.TableRows(row.RootPage), database.IndexRows(row.RootPage)} {
			measure(func() { _, _ = countRows(rows) })
		}
		if row.Type != "table" {
			continue
		}
		var table *Table
		measure(func() { table, _ = database.Table(row.Name) })
		if table != nil {
			measure(func() { _, _ = table.Select().OrderBy(table.Columns()[0].Name).All() })
		}
	}
	return most, int64(database.PageCount()) * int64(database.PageSize()), read
}

func TestMutatedFixturesReadWithBoundedWork(t *testing.T) {
	// Each walk reads every page at most once, so no walk reads more than
	// the pages times their size, and what a read allocates follows the
	// bytes it read.
	const (
		perFixture     = 3
		perByteRead    = 256
		allocatedFloor = 4 << 20
	)
	random := rand.New(rand.NewPCG(527, 3))
	for _, set := range fixtureSets(t) {
		t.Run("mutations of "+filepath.Base(set.name)+" read with bounded work", func(t *testing.T) {
			for range perFixture {
				main, log, journal := mutate(random, set.main), set.log, set.journal
				if log != nil && random.IntN(2) == 0 {
					log = withLogChecksums(mutate(random, log))
				}
				if journal != nil && random.IntN(2) == 0 {
					journal = mutate(random, journal)
				}
				var most, bound, read int64
				allocated := allocatedBytes(func() { most, bound, read = readMeasured(main, log, journal) })
				if most > bound {
					t.Errorf("a walk read %d bytes of a database of %d", most, bound)
				}
				if allocated > uint64(read)*perByteRead+allocatedFloor {
					t.Errorf("reading %d bytes allocated %d", read, allocated)
				}
			}
		})
	}
}
