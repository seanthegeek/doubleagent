package collect

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/walk"
)

func TestADiscoveredProjectWhoseDotDotNamesAJunctionOutIsNotWalked(t *testing.T) {
	root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	outside := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	for path, text := range map[string]string{
		root + "/Users/alice/.claude.json": `{"projects": {"C:\\Users\\alice\\nope\\..\\j": {}}}` + "\n",
		outside + "/CLAUDE.md":             "# analyst\n",
	} {
		parent := syspath.ToNative(path[:strings.LastIndexByte(path, '/')])
		if err := os.MkdirAll(parent, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(syspath.ToNative(path), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	makeJunction(t, outside, root+"/Users/alice/j")
	refusal, leaves := walk.NewRootCheck(root).Exit(root + "/Users/alice/nope/../j/CLAUDE.md")
	if !leaves || !syspath.Equal(refusal.Link, root+"/Users/alice/j") ||
		refusal.Kind != follow.LeadsOut {
		t.Errorf("Exit = %+v, %v; want the junction refused as leading out", refusal, leaves)
	}
	outDir := syspath.FromNative(t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), testOptions(root, outDir), testHost(t), &stdout,
		&stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	log, rows := runOutput(t, outDir, ".log"), runOutput(t, outDir, ".manifest.jsonl")
	if strings.Contains(rows, "CLAUDE.md") || strings.Contains(rows, syspath.ToNative(outside)) {
		t.Errorf("the manifest holds a row from outside the root:\n%s", rows)
	}
	notWalked := syspath.ToNative(root+"/Users/alice/j") +
		", which leads out of the image root; not walked"
	if !strings.Contains(log, notWalked) {
		t.Errorf("the log lacks %q:\n%s", notWalked, log)
	}
}

func TestADiscoveredProjectWithDotDotIsStagedUnderFsOnce(t *testing.T) {
	root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	name := root[strings.LastIndexByte(root, '/')+1:]
	claudeJSON := `{"projects": {"C:\\..\\` + name + `\\Users\\alice\\work": {},` +
		` "C:\\Users\\alice\\work": {}}}` + "\n"
	for path, text := range map[string]string{
		root + "/Users/alice/.claude.json":     claudeJSON,
		root + "/Users/alice/work/AGENTS.md":   "# work\n",
		root + "/Users/alice/work/.git/config": "",
	} {
		parent := syspath.ToNative(path[:strings.LastIndexByte(path, '/')])
		if err := os.MkdirAll(parent, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(syspath.ToNative(path), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	outDir := syspath.FromNative(t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), testOptions(root, outDir), testHost(t), &stdout,
		&stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	for member := range archiveMembers(t, outDir) {
		if slices.Contains(strings.Split(member, "/"), "..") ||
			strings.Contains(member, "AGENTS.md") && !strings.HasPrefix(member, "./fs/") {
			t.Errorf("archive member %s", member)
		}
	}
	rows, texts := manifestRows(t, outDir)
	agents := 0
	for i, row := range rows {
		archived, _ := row["archive_path"].(string)
		if slices.Contains(strings.Split(archived, "/"), "..") {
			t.Errorf("a manifest archive path holds ..: %s", texts[i])
		}
		if path, _ := row["path"].(string); strings.HasSuffix(path, `\AGENTS.md`) {
			agents++
			if path != syspath.ToNative(root+"/Users/alice/work/AGENTS.md") ||
				row["status"] != "collected" {
				t.Errorf("row %s", texts[i])
			}
		}
	}
	if agents != 1 {
		t.Errorf("%d rows of AGENTS.md, want 1:\n%s", agents, strings.Join(texts, "\n"))
	}
}

func TestAReparsePointThatIsNoLinkIsWordedAsOneAtEverySite(t *testing.T) {
	root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	if err := os.MkdirAll(syspath.ToNative(root+"/Users/alice"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(syspath.ToNative(root+"/Users/alice/notes"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	listenUnix(t, root, "Users/alice/.config", "Users/alice/.bashrc.d", "Users/bob", "var")
	outDir := syspath.FromNative(t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), testOptions(root, outDir), testHost(t), &stdout,
		&stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	log := runOutput(t, outDir, ".log")
	const reason = " is a reparse point other than a symlink or junction; " +
		"nothing behind it was looked at"
	native := func(path string) string { return syspath.ToNative(root + path) }
	for _, line := range []string{
		"user homes: " + native("/Users/bob") + reason + "\n",
		"Discovery [alice] " + native("/Users/alice/.config") + reason + "\n",
		native("/Users/alice/.config") + reason + ", not walked\n",
		"startup files: " + native("/Users/alice/.bashrc.d") + reason + "\n",
		"docker: " + native("/var") + reason + "\n",
	} {
		if !strings.Contains(log, line) {
			t.Errorf("the log lacks %q:\n%s", line, log)
		}
	}
	if strings.Contains(log, "leads out of the image root") {
		t.Errorf("the log calls a reparse point that is no link a link out:\n%s", log)
	}
}

// makeJunction makes link a junction to the directory target, both
// canonical, by setting on a new empty directory the mount-point reparse
// data mklink /J writes.
func makeJunction(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Mkdir(syspath.ToNative(link), 0o755); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(syspath.ToNative(link))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	native := syspath.ToNative(target)
	data := mountPointData(`\??\`+native+`\`, native)
	var returned uint32
	err = windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &data[0],
		uint32(len(data)), nil, 0, &returned, nil)
	if err != nil {
		t.Fatal(err)
	}
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

// listenUnix makes an AF_UNIX socket at each of names, relative to dir, from
// inside dir so that the name Go hands the system stays under its 108-byte
// limit. It skips the test only where the system does not support AF_UNIX
// sockets, and fails it on any other error.
func listenUnix(t *testing.T, dir string, names ...string) {
	t.Helper()
	t.Chdir(syspath.ToNative(dir))
	for _, name := range names {
		listener, err := net.Listen("unix", syspath.ToNative(name))
		if errors.Is(err, windows.WSAEAFNOSUPPORT) || errors.Is(err, windows.WSAEPROTONOSUPPORT) {
			t.Skipf("this system does not support AF_UNIX sockets: %v", err)
		}
		if err != nil {
			t.Fatalf("creating an AF_UNIX socket at %s in %s: %v", name, dir, err)
		}
		t.Cleanup(func() { _ = listener.Close() })
	}
}

// TestARunWithALowerCaseDriveRootCollects runs the whole chain, ResolveRoot
// through NewRootCheck to the copy, with -r spelled with its drive letter in
// lower case: every file under the root is still inside it.
func TestARunWithALowerCaseDriveRootCollects(t *testing.T) {
	root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	for path, text := range map[string]string{
		root + "/Users/alice/.claude/settings.json": "{}\n",
		root + "/Users/alice/.claude.json":          `{"projects": {"C:\\Users\\alice\\work": {}}}` + "\n",
		root + "/Users/alice/work/AGENTS.md":        "# work\n",
	} {
		parent := syspath.ToNative(path[:strings.LastIndexByte(path, '/')])
		if err := os.MkdirAll(parent, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(syspath.ToNative(path), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lower := strings.ToLower(root[:1]) + root[1:]
	outDir := syspath.FromNative(t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), testOptions(syspath.ToNative(lower), outDir),
		testHost(t), &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	log := runOutput(t, outDir, ".log")
	if strings.Contains(log, "outside the image root") {
		t.Errorf("the log refuses a path as outside the root:\n%s", log)
	}
	for _, path := range []string{"/Users/alice/.claude/settings.json", "/Users/alice/work/AGENTS.md"} {
		rows, texts := manifestRows(t, outDir)
		found := false
		for i, row := range rows {
			if native, _ := row["path"].(string); syspath.Equal(syspath.FromNative(native), root+path) {
				found = true
				if row["status"] != "collected" {
					t.Errorf("row %s", texts[i])
				}
			}
		}
		if !found {
			t.Errorf("no row of %s:\n%s", path, strings.Join(texts, "\n"))
		}
	}
}

func TestTwoHomesDifferingOnlyInCaseAreBothStaged(t *testing.T) {
	root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	markCaseSensitive(t, root+"/Users")
	texts := map[string]string{"Alice": `{"who": "Alice"}` + "\n", "alice": `{"who": "alice"}` + "\n"}
	for user, text := range texts {
		if err := os.MkdirAll(syspath.ToNative(root+"/Users/"+user), 0o755); err != nil {
			t.Fatal(err)
		}
		path := syspath.ToNative(root + "/Users/" + user + "/.claude.json")
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	outDir := syspath.FromNative(t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), testOptions(root, outDir), testHost(t), &stdout,
		&stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	members := archiveMembers(t, outDir)
	rows, lines := manifestRows(t, outDir)
	for user, text := range texts {
		path := syspath.ToNative(root + "/Users/" + user + "/.claude.json")
		sum := sha256.Sum256([]byte(text))
		found := false
		for i, row := range rows {
			if row["path"] != path {
				continue
			}
			found = true
			archived, _ := row["archive_path"].(string)
			if row["status"] != "collected" || row["sha256"] != hex.EncodeToString(sum[:]) ||
				members["./"+archived] != row["sha256"] {
				t.Errorf("%s's row %s; archive member %s holds %s", user, lines[i], archived,
					members["./"+archived])
			}
		}
		if !found {
			t.Errorf("no row of %s:\n%s", path, strings.Join(lines, "\n"))
		}
	}
}

// markCaseSensitive makes the directory path, canonical, and marks it
// case-sensitive, as WSL marks the directories it makes; the test is skipped
// where Windows refuses.
func markCaseSensitive(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(syspath.ToNative(path), 0o755); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.FILE_WRITE_ATTRIBUTES, windows.FILE_SHARE_READ|
		windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	flags := uint32(windows.FILE_CS_FLAG_CASE_SENSITIVE_DIR)
	if err := windows.SetFileInformationByHandle(handle, windows.FileCaseSensitiveInfo,
		(*byte)(unsafe.Pointer(&flags)), uint32(unsafe.Sizeof(flags))); err != nil {
		t.Skipf("cannot mark %s case-sensitive: %v", path, err)
	}
}

func TestAStagingDirectoryThatCannotBeMarkedRefusesTheLaterEntry(t *testing.T) {
	root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	markCaseSensitive(t, root+"/Users")
	for _, user := range []string{"Alice", "alice"} {
		if err := os.MkdirAll(syspath.ToNative(root+"/Users/"+user), 0o755); err != nil {
			t.Fatal(err)
		}
		path := syspath.ToNative(root + "/Users/" + user + "/.claude.json")
		if err := os.WriteFile(path, []byte(user+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	outDir := syspath.FromNative(t.TempDir())
	host := testHost(t)
	host.markCaseSensitive = func(string) error { return errors.New("not supported") }
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), testOptions(root, outDir), host, &stdout,
		&stderr); code != 0 {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	if log := runOutput(t, outDir, ".log"); !strings.Contains(log, foldingLine(outDir)+"\n") {
		t.Errorf("the log lacks %q:\n%s", foldingLine(outDir), log)
	}
	rows, lines := manifestRows(t, outDir)
	statuses := map[string]int{}
	for _, row := range rows {
		if path, _ := row["path"].(string); strings.HasSuffix(path, `\.claude.json`) {
			statuses[row["status"].(string)]++
		}
	}
	if statuses["collected"] != 1 || statuses["error_copy"] != 1 {
		t.Errorf("statuses %v, want one collected and one error_copy:\n%s", statuses,
			strings.Join(lines, "\n"))
	}
}
