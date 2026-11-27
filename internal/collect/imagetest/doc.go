// Package imagetest builds, for tests only, the fake disk image collect is
// tested against end to end, by replaying a committed manifest under a
// directory that becomes the image's work directory: testdata/image_unix.txt
// on Linux and macOS, testdata/image_windows.txt on Windows. Methods of Image
// build what single cases add to it or the separate roots they run over. A
// committed tree or archive could not hold the image: it has names holding "
// and \, links, sockets, named pipes, shut directories, junctions and deny
// rules.
//
// Each home of image_unix.txt holds only what a real install on one system
// writes there: alice and grace are Linux homes (grace the desktop apps'
// user-data directories whose names differ only in case from a CLI's in
// alice's), bob is a macOS home, heidi and ivan Windows homes, and the
// other homes each test one shape (links, odd names, an empty or service
// home). image_windows.txt follows the same rule with the Windows home
// alice holding what every system shares: ivan is a second Windows home,
// bob and grace Linux homes, judy a macOS home. The table of homes and
// systems is in manifest_layout_test.go, whose tests fail on a path in a
// home of the wrong system and on two entries differing only in ASCII case.
// CaseVariantRoot builds both spellings in one home on purpose, in both
// orders.
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
// The manifest is edited by hand: when a catalog line gains a fixture, add
// its file line to the home of each system the agent's research document
// places the path on, in sorted place (Build makes the directories above
// it), and run this package's tests, which fail on a line out of order or
// not as Format writes it.
//
// Build sets every entry's times and mode, deepest first, so making a child
// never moves its directory's time; the fixture builders do the same for
// what they add. Left to the tests' normalisation: change and birth times,
// owners, a link's own mode, and every mode on Windows. On Windows Build and
// the builders also settle the tree (Settle), so listings report the times
// the entries hold.
package imagetest
