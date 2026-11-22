// Package imagetest builds, for tests only, the fake disk image collect is
// tested against end to end, by replaying a committed manifest under a
// directory that becomes the image's work directory: testdata/image_unix.txt
// on Linux and macOS, testdata/image_windows.txt on Windows. Methods of Image
// build what single cases add to it or the separate roots they run over. A
// committed tree or archive could not hold the image: it has names holding "
// and \, names differing only in case, links, sockets, named pipes, shut
// directories, junctions and deny rules.
//
// Each line is a path below the work directory ("/" between names), a kind
// (see Kind), its argument, then name=value attributes (see Entry), single
// spaces apart: `"root/a b" file "hi\n" mode=0600`. Every
// path, text and target is a Go double-quoted string as strconv.Quote writes
// it, so " and \ are escaped with \, a byte that is not valid UTF-8 is \xNN
// and a control character its escape (\n): a line names its bytes exactly. A
// file's content is quoted parts or count*"text" repeats, so a large file is
// not stored. Lines starting with # are comments; entries stay sorted as
// LC_ALL=C sort orders the lines, and order does not change what is built.
// To add a fixture, add its file line in sorted place (Build makes the
// directories above it) and run this package's tests, which fail on a line
// out of order or not as Format writes it.
//
// Build sets every entry's times and mode, deepest first, so making a child
// never moves its directory's time; the fixture builders do the same for
// what they add. Left to the tests' normalisation: change and birth times,
// owners, a link's own mode, and every mode on Windows. A case-folding file
// system keeps one entry of each pair differing only in case, spelled as the
// one sorting first. On Windows Build and the builders also settle the tree
// (Settle), so listings report the times the entries hold.
package imagetest
