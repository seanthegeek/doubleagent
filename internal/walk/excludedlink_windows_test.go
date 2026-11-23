package walk

import (
	"context"
	"os"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

func TestExcludedLinkIsASymlinkRowNeverFollowedOnWindows(t *testing.T) {
	makers := []struct {
		name string
		make func(t *testing.T, target, link string)
	}{
		{"an excluded junction", makeJunction},
		{"an excluded directory symlink", func(t *testing.T, target, link string) {
			if err := os.Symlink(syspath.ToNative(target), syspath.ToNative(link)); err != nil {
				t.Skipf("a directory symlink needs the symlink privilege: %v", err)
			}
		}},
	}
	for _, maker := range makers {
		for _, throughWalk := range []bool{true, false} {
			name := maker.name + " met by the catalog match"
			if throughWalk {
				name = maker.name + " met by the walk"
			}
			t.Run(name, func(t *testing.T) {
				root := physicalTemp(t)
				writeFiles(t, root, "home/u/.tool/keep.txt", "home/u/elsewhere/deep/secret.txt")
				link := root + "/home/u/.tool/excluded"
				maker.make(t, root+"/home/u/elsewhere", link)
				collector, recorder := windowsCollector(t, root, []string{"*excluded*"})
				var err error
				if throughWalk {
					_, err = collector.CollectTree(context.Background(), "u", root+"/home/u",
						"agent", root+"/home/u/.tool")
				} else {
					_, err = collector.CollectPath(context.Background(), "u", root+"/home/u",
						"agent", link)
				}
				mustDo(t, err)
				want := 1
				if throughWalk {
					want = 2
				}
				if rows := recorder.rows(t); len(rows) != want {
					t.Fatalf("rows %v, want %d: nothing behind the link may be listed", rows, want)
				}
				row := recorder.rowOf(t, link)
				target, err := os.Readlink(syspath.ToNative(link))
				mustDo(t, err)
				meta, err := fsmeta.Lstat(syspath.ToNative(link))
				mustDo(t, err)
				if row["type"] != "symlink" || row["status"] != "skipped_excluded" ||
					row["target"] != target || row["size"] != float64(meta.Size) ||
					row["archive_path"] != "" {
					t.Errorf("row %v, want a skipped_excluded symlink row of the link itself, "+
						"target %q, size %d, no archive path", row, target, meta.Size)
				}
				names := stagedNames(t, collector.Stager.Dir)
				if names["excluded"] || names["secret.txt"] || names["keep.txt"] != throughWalk {
					t.Errorf("staged names %v, want the walk's keep.txt alone, never the link", names)
				}
			})
		}
	}
}
