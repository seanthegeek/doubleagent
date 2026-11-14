//go:build parity && windows

package parity

import (
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// Each setup function builds the fixtures one smoke.ps1 run builds before
// it, and registers their removal (or the undoing of an access rule) for the
// end of the case, so the next case sees the image as smoke.ps1 built it.

// setupWindowsEnvScanOnce adds one startup file and one file only the sweep
// reaches.
func setupWindowsEnvScanOnce(t *testing.T, img image) {
	build := fixture{t}
	build.file(img.alice+`\.bashrc.d\once.sh`, "export CODEX_HOME=/opt/x\n")
	build.file(img.alice+`\work\once.sh`, "export CODEX_HOME=/opt/x\n")
}

// setupGlobOutput makes the output directory whose name holds wildcard
// characters, in bob's home, which the sweep must still leave out.
func setupGlobOutput(t *testing.T, img image) {
	fixture{t}.dir(img.bob + `\out [1]`)
}

// setupWindowsDockerDesktop adds the Windows Docker Desktop disk directory.
func setupWindowsDockerDesktop(t *testing.T, img image) {
	fixture{t}.file(img.alice+`\AppData\Local\Docker\wsl\disk\readme.txt`, "vhdx lives here")
}

// setupWindowsRelativeProject builds the project a relative -p names.
func setupWindowsRelativeProject(t *testing.T, img image) {
	fixture{t}.file(img.root+`\srv\relproj\AGENTS.md`, "# rel")
}

// setupWindowsInventory sets the mtime the inventory's last field reports
// and gives bob a directory holding only a nested agent's entry.
func setupWindowsInventory(t *testing.T, img image) {
	build := fixture{t}
	build.touch(img.bob+`\.ollama\history`, time.Date(2026, 9, 15, 1, 2, 3, 0, time.UTC))
	build.file(img.bob+`\.gemini\antigravity-cli\conversations\b1.db`, "db")
}

// setupReadOnlyOutput makes an output directory in which the current user
// may not create files or directories.
func setupReadOnlyOutput(t *testing.T, img image) {
	directory := img.work + `\readonly`
	fixture{t}.dir(directory)
	denyAccess(t, directory, "(WD,AD)")
}

// setupWindowsUnreadableCaseVariant denies listing one spelling of a catalog
// directory whose other spelling is the same directory on NTFS. An enabled
// SeBackupPrivilege lets the listing through the deny rule, so the case is
// skipped there, as smoke.ps1 skips it.
func setupWindowsUnreadableCaseVariant(t *testing.T, img image) {
	if backupPrivilegeEnabled(t) {
		t.Skip("SeBackupPrivilege is enabled in this session and bypasses the deny rule")
	}
	denyAccess(t, img.alice+`\.config\pearai\User`, "(RD)")
}

// systemTool returns the path of a program in System32.
func systemTool(name string) string {
	return filepath.Join(os.Getenv("SystemRoot"), "System32", name)
}

// backupPrivilegeEnabled reports whether whoami /priv lists
// SeBackupPrivilege as enabled, its last column being the state.
func backupPrivilegeEnabled(t *testing.T) bool {
	t.Helper()
	output, err := exec.Command(systemTool("whoami.exe"), "/priv").Output()
	if err != nil {
		t.Fatalf("whoami /priv: %v", err)
	}
	return regexp.MustCompile(`(?m)^\s*SeBackupPrivilege\s.*\sEnabled\s*$`).Match(output)
}

// currentUser returns the security identifier of the user the test runs as.
func currentUser(t *testing.T) string {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	return user.User.Sid.String()
}

// denyAccess adds a rule denying the current user the given icacls rights on
// path, and removes the user's deny rules again at the end of the case.
func denyAccess(t *testing.T, path, rights string) {
	t.Helper()
	account := "*" + currentUser(t)
	t.Cleanup(func() {
		if output, err := exec.Command(systemTool("icacls.exe"), path, "/remove:d", account).CombinedOutput(); err != nil {
			t.Errorf("icacls /remove:d %s: %v\n%s", path, err, output)
		}
	})
	if output, err := exec.Command(systemTool("icacls.exe"), path, "/deny", account+":"+rights).CombinedOutput(); err != nil {
		t.Fatalf("icacls /deny %s: %v\n%s", path, err, output)
	}
}

// setupLinkImage builds the link image: a file and a directory outside the
// collection, and in eve's antigravity-cli directory a relative, a dangling
// and a directory symlink and a junction. The
// symlinks are left out, with a note, where the account may not create them.
func setupLinkImage(t *testing.T, img image) {
	build := fixture{t}
	root := img.work + `\linkroot`
	agent := root + `\home\eve\.gemini\antigravity-cli`
	build.file(agent+`\log\real.log`, "LINK-TARGET-BYTES")
	build.file(root+`\outside\secret.txt`, "OUTSIDE-BYTES")
	links := [][2]string{{`log\real.log`, `\cli.log`}, {`log\missing.log`, `\dangling.log`},
		{`..\..\..\..\outside`, `\out`}}
	for _, link := range links {
		if !symlinkOrNote(t, link[0], agent+link[1]) {
			break
		}
	}
	makeJunction(t, root+`\outside`, agent+`\jn`)
}

// symlinkOrNote creates a symlink at path to target and reports whether it
// could; where the account may not create symlinks it logs a note instead
// of failing, so the case runs with the rest of its image.
func symlinkOrNote(t *testing.T, target, path string) bool {
	t.Helper()
	err := os.Symlink(target, path)
	if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		t.Logf("note: symlinks cannot be created here, the case runs without them: %v", err)
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	return true
}

// makeJunction makes link a junction to the directory target, through
// FSCTL_SET_REPARSE_POINT on a fresh directory, as mklink /J does.
func makeJunction(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Mkdir(link, 0o755); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(link)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	data := mountPointData(`\??\`+target+`\`, target)
	var returned uint32
	if err := windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &data[0], uint32(len(data)),
		nil, 0, &returned, nil); err != nil {
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

// setupNonASCIIRoot builds the image of Latin-1, CJK and Cyrillic names: one
// profile's transcripts, a name holding a lone surrogate (written through
// Go's WTF-8 file names), and a symlink with a Cyrillic name to an absolute
// target with a CJK name, left out with a note where symlinks cannot be made.
func setupNonASCIIRoot(t *testing.T, img image) {
	build := fixture{t}
	directory := img.work + `\root71\Users\alice\.claude\projects\-C-u71`
	build.dir(directory)
	names := []string{"uümlé.jsonl", "日本 語.jsonl",
		"Журнал.jsonl", "lone\xed\xa0\x80.jsonl"}
	for _, name := range names {
		build.write(directory+`\`+name, `{"type":"user"}`)
	}
	symlinkOrNote(t, directory+`\`+names[1], directory+`\`+"ссылка.jsonl")
}
