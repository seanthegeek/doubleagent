//go:build unix

package walk

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/catalog/catalogtest"
	"github.com/seanthegeek/doubleagent/internal/collect/imagetest"
)

// rowAccessTime matches a row's access time. These runs restore no access
// time, so reading a file in one run moves the time the next run records.
var rowAccessTime = regexp.MustCompile(`"atime":[0-9]+`)

// parsedCatalog parses a catalog text for a test, failing it when the text
// does not parse.
func parsedCatalog(t *testing.T, text []byte) *catalog.Tables {
	t.Helper()
	tables, err := catalog.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	return tables
}

// collectedWith collects the homes of root with the catalog text given and
// returns its rows, each access time written as N, its log lines and a
// listing of what it staged.
func collectedWith(t *testing.T, root string, text []byte) ([]string, []string, []string) {
	t.Helper()
	run := collectHomesWith(t, root, parsedCatalog(t, text), edgeRuns[0])
	rows := make([]string, len(run.rows))
	for i, row := range run.rows {
		rows[i] = rowAccessTime.ReplaceAllString(row, `"atime":N`)
	}
	return rows, run.log, listStage(t, run.stage)
}

func TestTheOrderOfTheCatalogLinesChangesNoRowNoLogLineAndNoStagedFile(t *testing.T) {
	texts := map[string][]byte{
		"reversed":                   catalogtest.Reversed(catalog.Text()),
		"shuffled with seed 583":     catalogtest.Shuffled(catalog.Text(), 583),
		"shuffled with seed 2026112": catalogtest.Shuffled(catalog.Text(), 2026112),
	}
	images := map[string]string{
		"the fixture image": imagetest.Build(t, t.TempDir()).Root,
		"the edge image":    edgeImage(t),
	}
	for imageName, root := range images {
		wantRows, wantLog, wantStage := collectedWith(t, root, catalog.Text())
		if len(wantRows) < 100 && imageName == "the fixture image" {
			t.Fatalf("%s gave %d rows, too few to show anything", imageName, len(wantRows))
		}
		for textName, text := range texts {
			rows, log, staged := collectedWith(t, root, text)
			for _, output := range []struct {
				name      string
				got, want []string
			}{{"rows", rows, wantRows}, {"log lines", log, wantLog}, {"staged files", staged, wantStage}} {
				if !slices.Equal(output.got, output.want) {
					t.Errorf("%s, catalog %s: the %s differ:\n%s\nwant\n%s", imageName, textName, output.name,
						strings.Join(output.got, "\n"), strings.Join(output.want, "\n"))
				}
			}
		}
	}
}
