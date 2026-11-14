package walk

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"maps"
	"net"
	"os"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
	"unsafe"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// events records the manifest rows and the log lines of a walk in the order
// they are written, each row as "row <json>" and each line as "log <text>".
type events struct {
	written []string
}

// Line records a log line.
func (recorder *events) Line(text string) error {
	recorder.written = append(recorder.written, "log "+text)
	return nil
}

// rowSink is the manifest's output, recording each row it is given.
type rowSink struct {
	recorder *events
}

// Write records each row line of text.
func (sink rowSink) Write(text []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimSuffix(string(text), "\n"), "\n") {
		sink.recorder.written = append(sink.recorder.written, "row "+line)
	}
	return len(text), nil
}

// rows returns the recorded rows, parsed.
func (recorder *events) rows(t *testing.T) []map[string]any {
	t.Helper()
	var parsed []map[string]any
	for _, event := range recorder.written {
		if line, isRow := strings.CutPrefix(event, "row "); isRow {
			var row map[string]any
			mustDo(t, json.Unmarshal([]byte(line), &row))
			parsed = append(parsed, row)
		}
	}
	return parsed
}

// rowOf returns the recorded row of path, a canonical path, or fails.
func (recorder *events) rowOf(t *testing.T, path string) map[string]any {
	t.Helper()
	for _, row := range recorder.rows(t) {
		if row["path"] == syspath.ToNative(path) {
			return row
		}
	}
	t.Fatalf("no row of %s in %q", path, recorder.written)
	return nil
}

// mustDo fails the test on err.
func mustDo(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// physicalTemp returns a fresh temporary directory, canonical and resolved,
// so that paths under it are spelled as the system resolves them.
func physicalTemp(t *testing.T) string {
	t.Helper()
	physical, err := stage.PhysicalDir(syspath.FromNative(t.TempDir()))
	mustDo(t, err)
	return physical
}

// writeFiles creates each file under base, with its parents, holding its
// own name.
func writeFiles(t *testing.T, base string, files ...string) {
	t.Helper()
	for _, file := range files {
		path := base + "/" + file
		mustDo(t, os.MkdirAll(path[:strings.LastIndexByte(path, '/')], 0o755))
		mustDo(t, os.WriteFile(path, []byte(file), 0o644))
	}
}

// windowsCollector returns a collector over the image root root with a fresh
// staging directory, rows and log lines recorded in one sequence.
func windowsCollector(t *testing.T, root string, excludes []string) (*Collector, *events) {
	t.Helper()
	stageDir := physicalTemp(t)
	recorder := &events{}
	return &Collector{
		Tables: &catalog.Tables{}, Excludes: excludes, Secret: MatchAny(nil), Log: recorder,
		Manifest: manifest.NewWriter(rowSink{recorder}), Root: root, Owners: fsmeta.NewOwners(),
		Stager: stage.Stager{Dir: stageDir, Physical: stageDir, Root: root},
	}, recorder
}

// makeJunction makes link a junction to the directory target without
// running a program: it creates an empty directory and sets on it the
// mount-point reparse data mklink /J would write. Both paths are canonical
// and absolute.
func makeJunction(t *testing.T, target, link string) {
	t.Helper()
	mustDo(t, os.Mkdir(link, 0o755))
	name, err := windows.UTF16PtrFromString(syspath.ToNative(link))
	mustDo(t, err)
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	mustDo(t, err)
	defer windows.CloseHandle(handle)
	native := syspath.ToNative(target)
	data := mountPointData(`\??\`+native+`\`, native)
	var returned uint32
	mustDo(t, windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &data[0],
		uint32(len(data)), nil, 0, &returned, nil))
}

// mountPointData lays out a REPARSE_DATA_BUFFER holding a mount-point
// (junction) reparse buffer: the tag, the data length, a reserved word, the
// offsets and lengths in bytes of the two names, then the substitute name
// the system follows and the print name shown to people, each ending in NUL.
func mountPointData(substitute, printName string) []byte {
	substituteText := utf16.Encode([]rune(substitute))
	printText := utf16.Encode([]rune(printName))
	names := make([]uint16, 0, len(substituteText)+len(printText)+2)
	names = append(append(names, substituteText...), 0)
	names = append(append(names, printText...), 0)
	substituteBytes := uint16(2 * len(substituteText))
	data := binary.LittleEndian.AppendUint32(nil, windows.IO_REPARSE_TAG_MOUNT_POINT)
	data = binary.LittleEndian.AppendUint16(data, uint16(8+2*len(names)))
	data = binary.LittleEndian.AppendUint16(data, 0)
	data = binary.LittleEndian.AppendUint16(data, 0)
	data = binary.LittleEndian.AppendUint16(data, substituteBytes)
	data = binary.LittleEndian.AppendUint16(data, substituteBytes+2)
	data = binary.LittleEndian.AppendUint16(data, uint16(2*len(printText)))
	for _, unit := range names {
		data = binary.LittleEndian.AppendUint16(data, unit)
	}
	return data
}

// holdExclusively opens path sharing nothing until the test ends, so that
// a copy's read open fails with a sharing violation while its lstat, which
// asks for no access, still succeeds.
func holdExclusively(t *testing.T, path string) {
	t.Helper()
	name, err := windows.UTF16PtrFromString(syspath.ToNative(path))
	mustDo(t, err)
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL, 0)
	mustDo(t, err)
	t.Cleanup(func() { _ = windows.CloseHandle(handle) })
}

// tokenOwner returns the account name of the owner this process gives the
// files it creates, the token's default owner.
func tokenOwner(t *testing.T) string {
	t.Helper()
	token := windows.GetCurrentProcessToken()
	var size uint32
	_ = windows.GetTokenInformation(token, windows.TokenOwner, nil, 0, &size)
	information := make([]byte, size)
	mustDo(t, windows.GetTokenInformation(token, windows.TokenOwner, &information[0], size, &size))
	owner := (*struct{ Owner *windows.SID })(unsafe.Pointer(&information[0])).Owner
	account, domain, _, err := owner.LookupAccount("")
	mustDo(t, err)
	if domain == "" {
		return account
	}
	return domain + `\` + account
}

// summary returns each recorded event as "row <path> <status>" or
// "log <first words>", with paths relative to base, for order checks.
func (recorder *events) summary(t *testing.T, base string) []string {
	t.Helper()
	prefix := syspath.ToNative(base) + `\`
	var shown []string
	for _, event := range recorder.written {
		if line, isRow := strings.CutPrefix(event, "row "); isRow {
			var row map[string]any
			mustDo(t, json.Unmarshal([]byte(line), &row))
			path := strings.TrimPrefix(row["path"].(string), prefix)
			shown = append(shown, "row "+path+" "+row["status"].(string))
			continue
		}
		line := strings.TrimPrefix(strings.ReplaceAll(event, prefix, ""), "log ")
		text, _, _ := strings.Cut(line, ": open")
		shown = append(shown, "log "+text)
	}
	return shown
}

func TestRowsFollowTheWalkOnWindows(t *testing.T) {
	home := physicalTemp(t)
	writeFiles(t, home, "a/1.txt", "a/2.locked", "a/3cache/f", "a/4.txt", "a/sub/5.txt")
	holdExclusively(t, home+"/a/2.locked")
	collector, recorder := windowsCollector(t, "", []string{"a/3cache"})
	_, err := collector.CollectPath(context.Background(), "u", home, "agent", home+"/a")
	mustDo(t, err)
	want := []string{
		`row a\1.txt collected`,
		`log copy failed: a\2.locked`,
		`row a\2.locked error_copy`,
		`row a\3cache skipped_excluded`,
		`row a\4.txt collected`,
		`row a\sub\5.txt collected`,
	}
	if got := recorder.summary(t, home); !slices.Equal(got, want) {
		t.Errorf("events\n%q\nwant\n%q", got, want)
	}
}

func TestClaimsAreFoldedOnWindows(t *testing.T) {
	home := physicalTemp(t)
	writeFiles(t, home, ".tool/a.txt", ".tool/nested/b.txt")
	cases := []struct {
		name  string
		claim string
	}{
		{"a claim spelled as on disk prunes its path", home + "/.tool/nested"},
		{"a claim spelled in other case prunes its path", home + "/.TOOL/NESTED"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			collector, recorder := windowsCollector(t, "", nil)
			owner := scope{user: "u", home: home, agent: "agent", stager: collector.Stager,
				claims: map[string]struct{}{syspath.Fold(testCase.claim): {}}}
			_, err := collector.collectTree(context.Background(), owner, home+"/.tool")
			mustDo(t, err)
			want := []string{`row .tool\a.txt collected`}
			if got := recorder.summary(t, home); !slices.Equal(got, want) {
				t.Errorf("events %q, want %q", got, want)
			}
			if !claimed(home+"/.tool/Nested/b.txt", home, owner.claims) {
				t.Error("a path under the claim in a third spelling is not claimed")
			}
		})
	}
}

func TestNestedLineInOtherCaseIsCollectedOnce(t *testing.T) {
	home := physicalTemp(t)
	writeFiles(t, home, ".tool/a.txt", ".tool/nested/b.txt")
	collector, recorder := windowsCollector(t, "", nil)
	table := []catalog.Entry{{Agent: "outer", Glob: ".tool"},
		{Agent: "inner", Glob: ".TOOL/NESTED"}}
	mustDo(t, collector.CollectCatalog(context.Background(), "u", home, table, ""))
	agents := map[string]string{}
	for _, row := range recorder.rows(t) {
		agents[row["path"].(string)] += row["agent"].(string) + " "
	}
	want := map[string]string{
		syspath.ToNative(home + "/.tool/a.txt"):        "outer ",
		syspath.ToNative(home + "/.tool/nested/b.txt"): "inner ",
	}
	if !maps.Equal(agents, want) {
		t.Errorf("rows by path %q, want %q", agents, want)
	}
}

func TestCaseVariantLinesGiveOneRowOnWindows(t *testing.T) {
	home := physicalTemp(t)
	writeFiles(t, home, ".tool/f.txt")
	collector, recorder := windowsCollector(t, "", nil)
	table := []catalog.Entry{{Agent: "first", Glob: ".tool/f.txt"},
		{Agent: "second", Glob: ".TOOL/F.TXT"}}
	mustDo(t, collector.CollectCatalog(context.Background(), "u", home, table, ""))
	native := syspath.ToNative(home + "/.tool/f.txt")
	want := []string{
		"log   [first] " + native,
		"row " + native + " collected",
		"log   [second] " + native + ": catalog line .TOOL/F.TXT names the same path as an" +
			" earlier line (case-insensitive filesystem), collected there",
	}
	if got := recorder.summary(t, "Q:"); !slices.Equal(got, want) {
		t.Errorf("events\n%q\nwant\n%q", got, want)
	}
}

func TestLinkNotRecreatedOnWindows(t *testing.T) {
	root := physicalTemp(t)
	writeFiles(t, root, "home/u/target/x.txt")
	mustDo(t, os.MkdirAll(root+"/home/u/.tool", 0o755))
	link := root + "/home/u/.tool/j"
	makeJunction(t, root+"/home/u/target", link)
	collector, recorder := windowsCollector(t, root, nil)
	// The staged link's directory is a file, so the link cannot be made.
	writeFiles(t, collector.Stager.Dir, "fs/home/u/.tool")
	_, err := collector.CollectPath(context.Background(), "u", root+"/home/u", "agent", link)
	mustDo(t, err)
	target, err := os.Readlink(link)
	mustDo(t, err)
	row := recorder.rowOf(t, link)
	reason, found := strings.CutPrefix(row["error"].(string), "not recreated in the archive: ")
	if row["status"] != "symlink" || row["type"] != "symlink" || !found || reason == "" ||
		row["archive_path"] != "" || row["target"] != target {
		t.Errorf("row %v, want a symlink row with its reason, target and no archive path", row)
	}
	wantLine := "log symlink not recreated: " + syspath.ToNative(link) + ": " + reason
	if !slices.Contains(recorder.written, wantLine) {
		t.Errorf("events %q lack %q", recorder.written, wantLine)
	}
}

func TestJunctionIsASymlinkRowNeverDescended(t *testing.T) {
	root := physicalTemp(t)
	writeFiles(t, root, "home/u/.tool/a.txt", "home/u/elsewhere/secret.txt")
	link := root + "/home/u/.tool/j"
	makeJunction(t, root+"/home/u/elsewhere", link)
	collector, recorder := windowsCollector(t, root, nil)
	_, err := collector.CollectPath(context.Background(), "u", root+"/home/u", "agent",
		root+"/home/u/.tool")
	mustDo(t, err)
	if rows := recorder.rows(t); len(rows) != 2 {
		t.Fatalf("rows %v, want a.txt and the junction only", rows)
	}
	row := recorder.rowOf(t, link)
	if row["type"] != "symlink" || row["status"] != "symlink" {
		t.Errorf("junction row %v, want type and status symlink", row)
	}
	if message, _ := row["error"].(string); strings.Contains(message, "privilege") {
		t.Skipf("recreating a directory link needs the symlink privilege: %s", message)
	}
	if row["archive_path"] != "fs/home/u/.tool/j" || row["error"] != "" {
		t.Errorf("junction row %v, want it recreated at fs/home/u/.tool/j", row)
	}
	staged, err := fsmeta.Lstat(collector.Stager.Dir + "/fs/home/u/.tool/j")
	if err != nil || !staged.DirectoryLink() {
		t.Errorf("staged link %+v, %v; want a directory link", staged, err)
	}
}

func TestSocketIsSkippedAsOtherOnWindows(t *testing.T) {
	home := physicalTemp(t)
	listener, err := net.Listen("unix", syspath.ToNative(home+"/s"))
	if err != nil {
		t.Skipf("creating an AF_UNIX socket needs Windows 10 1803 or later: %v", err)
	}
	defer listener.Close()
	collector, recorder := windowsCollector(t, "", nil)
	_, err = collector.CollectPath(context.Background(), "u", home, "agent", home+"/s")
	mustDo(t, err)
	row := recorder.rowOf(t, home+"/s")
	if row["status"] != "skipped_special" || row["type"] != "other" || row["archive_path"] != "" {
		t.Errorf("socket row %v, want skipped_special of type other", row)
	}
}

func TestOwnerAndAttributesPerRowKind(t *testing.T) {
	home := physicalTemp(t)
	writeFiles(t, home, ".tool/file.txt", ".tool/hidden.txt", ".tool/cache/x", "plain.txt")
	hidden, err := windows.UTF16PtrFromString(syspath.ToNative(home + "/.tool/hidden.txt"))
	mustDo(t, err)
	mustDo(t, windows.SetFileAttributes(hidden, windows.FILE_ATTRIBUTE_HIDDEN))
	collector, recorder := windowsCollector(t, "", []string{".tool/cache"})
	_, err = collector.CollectPath(context.Background(), "u", home, "agent", home+"/.tool")
	mustDo(t, err)
	mustDo(t, collector.WriteReadError("u", home, home+"/.tool", "denied"))
	mustDo(t, collector.WriteReadError("u", home, home+"/plain.txt", "not a directory"))
	mustDo(t, collector.WriteReadError("u", home, home+"/missing", "missing"))
	owner := tokenOwner(t)
	cases := []struct {
		name, path, status, owner, attributes string
	}{
		{"a collected file has its owner", ".tool/file.txt", "collected", owner, "Archive"},
		{"a hidden file says so", ".tool/hidden.txt", "collected", owner, "Hidden"},
		{"an excluded directory has no owner", ".tool/cache", "skipped_excluded", "", "Directory"},
		{"an unreadable directory has its owner", ".tool", "error_read", owner, "Directory"},
		{"a file read as a directory has none", "plain.txt", "error_read", "", "Archive"},
		{"a missing path has neither", "missing", "error_read", "", ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			row := rowWithStatus(t, recorder, home+"/"+testCase.path, testCase.status)
			if row["owner"] != testCase.owner || row["attributes"] != testCase.attributes {
				t.Errorf("row %v, want owner %q and attributes %q", row, testCase.owner,
					testCase.attributes)
			}
		})
	}
}

// rowWithStatus returns the recorded row of path with status, or fails.
func rowWithStatus(t *testing.T, recorder *events, path, status string) map[string]any {
	t.Helper()
	for _, row := range recorder.rows(t) {
		if row["path"] == syspath.ToNative(path) && row["status"] == status {
			return row
		}
	}
	t.Fatalf("no %s row of %s in %q", status, path, recorder.written)
	return nil
}

func TestCollectedRowsOnWindows(t *testing.T) {
	home := physicalTemp(t)
	writeFiles(t, home, ".tool/file.txt")
	collector, recorder := windowsCollector(t, "", nil)
	_, err := collector.CollectPath(context.Background(), "u", home, "agent", home+"/.tool/file.txt")
	mustDo(t, err)
	row := recorder.rowOf(t, home+"/.tool/file.txt")
	meta, err := fsmeta.Lstat(home + "/.tool/file.txt")
	mustDo(t, err)
	if row["size"] != float64(len(".tool/file.txt")) || row["mode"] != "" || row["uid"] != 0.0 ||
		row["ctime"] != 0.0 || row["mtime"] != float64(meta.Mtime) || row["btime"] != float64(meta.Btime) {
		t.Errorf("row %v, want the bytes copied, the times and no mode, uid or ctime", row)
	}
}

func TestProjectsThroughJunctionsOnWindows(t *testing.T) {
	root := physicalTemp(t)
	outside := physicalTemp(t)
	writeFiles(t, root, "home/dot/dotfiles/claude/proj/AGENTS.md")
	writeFiles(t, outside, "proj/AGENTS.md")
	makeJunction(t, root+"/home/dot/dotfiles/claude", root+"/home/dot/.claude")
	makeJunction(t, outside, root+"/home/dot/ext")
	if name, out := ProjectOutOfRoot(root, root+"/home/dot/ext/proj"); !out ||
		name != root+"/home/dot/ext" {
		t.Errorf("ProjectOutOfRoot through a junction out of the root = %s, %v", name, out)
	}
	if name, out := ProjectOutOfRoot(root, root+"/home/dot/.claude/proj"); out {
		t.Errorf("ProjectOutOfRoot through a junction inside the root = %s, %v", name, out)
	}
	collector, recorder := windowsCollector(t, root, nil)
	collector.Tables.ProjectCatalog = []catalog.Entry{{Agent: "agents", Glob: "AGENTS.md"}}
	project := root + "/home/dot/.claude/proj"
	mustDo(t, collector.CollectProject(context.Background(), "dot", root+"/home/dot", project))
	row := recorder.rowOf(t, project+"/AGENTS.md")
	if row["archive_path"] != "fs/home/dot/dotfiles/claude/proj/AGENTS.md" {
		t.Errorf("row %v, want it staged where the home holds it", row)
	}
	wantLine := "log Project [dot] " + syspath.ToNative(project) + " (through the symlinked directory " +
		syspath.ToNative(root+"/home/dot/.claude") + ", archived under " +
		syspath.ToNative(root+"/home/dot/dotfiles/claude/proj") + ")"
	if !slices.Contains(recorder.written, wantLine) {
		t.Errorf("events %q lack %q", recorder.written, wantLine)
	}
}

// customTag is a reparse tag no Microsoft filter owns, standing in for a
// cloud placeholder: neither a symlink nor a junction.
const customTag = 0x00001234

// customReparseData lays out a REPARSE_GUID_DATA_BUFFER for customTag: the
// tag, the data length, a reserved word, the owner's GUID and the data.
func customReparseData(payload []byte) []byte {
	data := binary.LittleEndian.AppendUint32(nil, customTag)
	data = binary.LittleEndian.AppendUint16(data, uint16(len(payload)))
	data = binary.LittleEndian.AppendUint16(data, 0)
	data = append(data, []byte("doubleagent-test")...)
	return append(data, payload...)
}

// controlReparsePoint opens path itself for writing and applies control to
// it with data, returning the error.
func controlReparsePoint(path string, control uint32, data []byte) error {
	name, err := windows.UTF16PtrFromString(syspath.ToNative(path))
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	var returned uint32
	return windows.DeviceIoControl(handle, control, &data[0], uint32(len(data)), nil, 0,
		&returned, nil)
}

// makeCustomReparsePoint sets customTag on path, an existing file or empty
// directory, skipping the test when the system refuses, and removes the tag
// again when the test ends so the temporary directory can be removed.
func makeCustomReparsePoint(t *testing.T, path string) {
	t.Helper()
	err := controlReparsePoint(path, windows.FSCTL_SET_REPARSE_POINT, customReparseData([]byte{1}))
	if err != nil {
		t.Skipf("setting a third-party reparse tag on %s: %v", path, err)
	}
	t.Cleanup(func() {
		_ = controlReparsePoint(path, windows.FSCTL_DELETE_REPARSE_POINT, customReparseData(nil))
	})
}

// fileRecords keeps the files an inventory walk counts.
type fileRecords struct{ sizes []int64 }

// Match ignores a catalog match.
func (records *fileRecords) Match(string, string) {}

// File keeps a file's size.
func (records *fileRecords) File(_ string, size, _ int64) {
	records.sizes = append(records.sizes, size)
}

func TestOtherReparsePointsAreLinksNeverEntered(t *testing.T) {
	home := physicalTemp(t)
	writeFiles(t, home, ".tool/a.txt", ".tool/placeholder.txt")
	directory := home + "/.tool/placeholder"
	mustDo(t, os.Mkdir(directory, 0o755))
	makeCustomReparsePoint(t, directory)
	makeCustomReparsePoint(t, home+"/.tool/placeholder.txt")
	if !isSymlink(directory) {
		t.Error("a directory with another reparse tag is not a link")
	}
	if links, outside := LinkedParent(home, directory+"/x"); !outside && links.From != directory {
		t.Errorf("LinkedParent through the placeholder = %+v, %v; want it taken as a link",
			links, outside)
	}
	collector, recorder := windowsCollector(t, "", nil)
	_, err := collector.CollectPath(context.Background(), "u", home, "agent", home+"/.tool")
	mustDo(t, err)
	want := []string{`row .tool\a.txt collected`, `row .tool\placeholder skipped_special`,
		`row .tool\placeholder.txt skipped_special`}
	if got := recorder.summary(t, home); !slices.Equal(got, want) {
		t.Errorf("events %q, want %q", got, want)
	}
	inventory, _ := windowsCollector(t, "", nil)
	records := &fileRecords{}
	inventory.Inventory, inventory.Records = true, records
	_, err = inventory.CollectPath(context.Background(), "u", home, "agent", home+"/.tool")
	mustDo(t, err)
	if !slices.Equal(records.sizes, []int64{int64(len(".tool/a.txt"))}) {
		t.Errorf("inventory sizes %v, want a.txt only", records.sizes)
	}
}
