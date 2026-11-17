package follow

// The guard's lists. reviewedImports names every standard and vendored
// package a non-test file of the binary may import; a guarded one's
// exported names may be used only when safeNames, noFollowNames or
// allowedCalls lists them.

// reviewedImports are the standard and vendored packages the binary may
// import outside the owner packages, each marked: "no path" for one that
// takes no file-system path, or works on one as text or on an fs.FS value it
// is given; "guarded" for one that can act on a path, whose names are
// checked one by one. An import not listed fails, so a new one is reviewed.
// Packages that open a path by name and that nothing imports today
// (debug/elf, debug/macho, debug/pe, debug/plan9obj, debug/buildinfo,
// plugin, net with its Unix sockets, io/ioutil, archive/zip) are not listed,
// so importing one fails.
func reviewedImports() map[string]string {
	noPath := []string{"archive/tar", "bufio", "bytes", "compress/gzip", "context",
		"crypto/rand", "crypto/sha256", "embed", "encoding/binary", "encoding/hex", "errors",
		"fmt", "io", "io/fs", "math", "os/signal", "path", "regexp", "runtime", "runtime/debug",
		"slices", "strconv", "strings", "testing", "time", "unicode", "unicode/utf16", "unicode/utf8"}
	guarded := []string{"os", "os/exec", "os/user", "path/filepath", "syscall",
		"golang.org/x/sys/unix", "golang.org/x/sys/windows", "golang.org/x/sys/windows/registry"}
	reviewed := map[string]string{}
	for _, name := range noPath {
		reviewed[name] = "no path"
	}
	for _, name := range guarded {
		reviewed[name] = "guarded"
	}
	return reviewed
}

// guardedModulePackages are this module's packages whose names are checked
// like a guarded package's: the ones follow is built on.
func guardedModulePackages() []string {
	return []string{"github.com/seanthegeek/doubleagent/internal/osfile",
		"github.com/seanthegeek/doubleagent/internal/fsmeta",
		"github.com/seanthegeek/doubleagent/internal/stage"}
}

// ownerPackages are the directories, exactly, whose files may use any name:
// this package, the path wrappers it is built on, the metadata layer whose
// functions it wraps, and the staging layer, which works on the output
// directory and resolves the root the run is given.
func ownerPackages() []string {
	return []string{"internal/follow", "internal/osfile", "internal/fsmeta", "internal/stage"}
}

// safeNames are, by import path, the exported names of guarded packages that
// take no file-system path or work on one as text alone: constants, types,
// error values, and functions such as os.Getenv or filepath.Join. A name that
// takes a path belongs on noFollowNames or allowedCalls instead.
func safeNames() map[string][]string {
	return map[string][]string{
		// The process, its environment and standard streams, the null
		// device's name, file types and flags. Getwd takes no path; the
		// operating system answers it from the process, though the Unix
		// fallback stats "." and the working directory to check $PWD.
		// SameFile compares two FileInfo values already read.
		"os": {"Args", "DevNull", "Environ", "Exit", "File", "FileInfo", "Getenv", "Getuid",
			"Getwd", "Hostname", "Interrupt", "LinkError", "ModeSymlink", "O_CREATE", "O_EXCL",
			"O_TRUNC", "O_WRONLY", "SameFile", "Stderr", "Stdout"},
		// The running account's own record, looked up by the system.
		"os/user": {"Current"},
		// Text operations on a path; none touches the file system.
		"path/filepath": {"Base", "Clean", "IsAbs", "Join", "SplitList", "VolumeName"},
		// Constants, a signal and an attribute record type.
		"syscall": {"ERROR_MORE_DATA", "FILE_ATTRIBUTE_DIRECTORY", "FILE_ATTRIBUTE_REPARSE_POINT",
			"SIGTERM", "Win32FileAttributeData"},
		// Constants, the umask, the kernel's name and a byte conversion.
		"golang.org/x/sys/unix": {"AT_EACCESS", "AT_FDCWD", "ByteSliceToString", "Umask", "Uname",
			"Utsname", "W_OK", "X_OK"},
		// Security identifiers, the version call, a token type, constants and
		// string conversions.
		"golang.org/x/sys/windows": {"CreateWellKnownSid", "ERROR_INVALID_NAME",
			"FILE_ATTRIBUTE_DIRECTORY", "MAX_LONG_PATH", "RtlGetVersion", "Token",
			"UTF16PtrFromString", "UTF16ToString", "WinBuiltinAdministratorsSid"},
		// Registry keys and values, which are not file paths.
		"golang.org/x/sys/windows/registry": {"DWORD", "ENUMERATE_SUB_KEYS", "EXPAND_SZ",
			"ErrNotExist", "ErrUnexpectedType", "ExpandString", "Key", "LOCAL_MACHINE", "OpenKey",
			"QUERY_VALUE", "READ", "SZ", "USERS"},
		// Types, kinds, constructors and conversions of metadata already read;
		// ListingOrder sorts entries it is given. The Opener's and Owners'
		// methods take paths and do not follow their last component. A
		// Reparse and the two link tags are reparse data already read.
		"github.com/seanthegeek/doubleagent/internal/fsmeta": {"AttributesString", "Device", "Dir",
			"Fifo", "File", "Kind", "ListingOrder", "Meta", "NewOpener", "NewOwners", "Opener",
			"Owners", "Reparse", "Socket", "Symlink", "TagMountPoint", "TagSymlink", "ZeroMeta"},
		// The staging type and a text comparison of two paths.
		"github.com/seanthegeek/doubleagent/internal/stage": {"Stager", "Under", "NewPlaces",
			"IsCollision"},
	}
}

// noFollowNames are names that take a path, never follow its last component
// and change nothing: an lstat, a readlink, or a read of a reparse point's
// own data (opened with FILE_FLAG_OPEN_REPARSE_POINT). They are everywhere
// and sound wherever they are, so they are not counted.
func noFollowNames() map[string][]string {
	return map[string][]string{
		"github.com/seanthegeek/doubleagent/internal/osfile": {"Lstat", "Readlink"},
		"github.com/seanthegeek/doubleagent/internal/fsmeta": {"Lstat", "ReadReparse"},
	}
}

// allowance is how many calls of one guarded name one file may make, and
// why each is sound there.
type allowance struct {
	count  int
	reason string
}

// allowedCalls are every other use of a guarded name that takes a path
// (one that follows, writes, removes, lists or resolves), keyed by file and
// name, with their number and reason: a further call in the same file fails
// too, and so does a changed count.
func allowedCalls() map[string]allowance {
	allowed := map[string]allowance{}
	for _, entry := range []struct {
		file, call string
		count      int
		reason     string
	}{
		{"internal/archive/archive.go", "osfile.Remove", 1,
			"removes a partial archive the run wrote"},
		{"internal/collect/run_setup.go", "osfile.Remove", 1,
			"removes the probe file it made in the output directory"},
		{"internal/collect/run_setup.go", "osfile.RemoveAll", 1,
			"removes the run's own staging directory"},
		{"internal/collect/run_finish.go", "osfile.Remove", 1,
			"removes an old file at the run's archive path before writing it"},
		{"internal/collect/options.go", "fsmeta.OnDiskPath", 1,
			"respells -p below the root, stopping at the first link"},
		{"internal/collect/options_windows.go", "fsmeta.OnDiskPath", 1,
			"respells -r and -o, the operator's own paths"},
		{"internal/collect/run_windows.go", "fsmeta.OnDiskPath", 1,
			"respells the ProgramData directory the environment names"},
		{"internal/walk/rootrules_windows.go", "fsmeta.OnDiskPath", 1,
			"respells a link target matching the root only in ASCII case, from the drive"},
		{"internal/projects/paths.go", "fsmeta.OnDiskPath", 1,
			"respells a discovered value below the root, stopping at the first link"},
		{"internal/users/users_windows.go", "fsmeta.OnDiskPath", 1,
			"respells a live ProfileList path from its drive, stopping at the first link"},
		{"internal/glob/respell_windows.go", "fsmeta.OnDiskName", 1,
			"looks a name up in a directory the root check let through"},
		{"internal/walk/files.go", "fsmeta.DiskUsageBytes", 1,
			"sizes an excluded directory under a checked base, each directory opened without " +
				"following a link and admitted"},
		{"internal/volumes/volumes.go", "fsmeta.DiskUsageBytes", 1,
			"sizes a volume's _data under checked parents, each directory opened without " +
				"following a link and admitted"},
		{"internal/hostcmd/hostcmd.go", "exec.CommandContext", 1,
			"runs getent, dscl or plutil, the one place the binary runs a program"},
		{"internal/hostcmd/hostcmd.go", "exec.LookPath", 2,
			"finds those programs in the system directories, then PATH"},
		{"internal/hostcmd/hostcmd.go", "exec.Error", 1,
			"builds the not-found error for a program"},
		{"internal/archive/archive.go", "osfile.Create", 1,
			"creates the archive in the output directory"},
		{"internal/archive/archive.go", "osfile.WriteFile", 1,
			"writes the archive's checksum file beside it"},
		{"internal/collect/run_setup.go", "osfile.MkdirAll", 2,
			"creates the output and staging directories"},
		{"internal/collect/run_setup.go", "osfile.Create", 1,
			"creates the manifest in the output directory"},
		{"internal/collect/run_setup.go", "stage.MarkCaseSensitive", 1,
			"marks the run's own new staging fs directory case-sensitive"},
		{"internal/collect/run_setup.go", "stage.FoldsCase", 1,
			"probes the run's own staging fs directory with a file it creates and removes"},
		{"internal/collect/run_finish.go", "osfile.WriteFile", 1,
			"writes collection.json into the output and staging directories"},
		{"internal/shortname/shortname_windows.go", "windows.GetShortPathName", 1,
			"names a test directory the helper made under the test's temporary directory"},
		{"internal/shortname/shortname_windows.go", "windows.GetLongPathName", 1,
			"names a test directory the helper made under the test's temporary directory"},
		{"internal/shortname/shortname_windows.go", "osfile.MkdirAll", 1,
			"makes a test directory under the test's own temporary directory"},
		{"internal/archive/archive.go", "osfile.ReadDir", 1,
			"lists the staging tree the run built"},
		{"internal/archive/archive.go", "osfile.Open", 2,
			"reads files of the staging tree the run built"},
		{"internal/collect/options.go", "osfile.Stat", 1,
			"tests -r and -o as the operator gives them"},
		{"internal/collect/run_finish.go", "osfile.Open", 1,
			"copies the run's own output files"},
		{"internal/collect/run_finish.go", "osfile.OpenFile", 1,
			"creates the run's own output files"},
		{"internal/collect/run_setup.go", "osfile.OpenFile", 1,
			"creates the probe file in the output directory"},
		{"internal/collect/run_setup.go", "stage.PhysicalDir", 1,
			"resolves the staging directory"},
		{"internal/collect/run_unix.go", "unix.Faccessat", 1,
			"tests the output directory -o names"},
		{"internal/glob/expand.go", "osfile.Stat", 1,
			"asked the root check of the path first"},
		{"internal/glob/expand.go", "fsmeta.OpenDir", 1,
			"asked the root check of the directory first; the opener checks the handle"},
		{"internal/projects/files.go", "fsmeta.OpenDir", 1,
			"opens a directory its own lstat typed one, below a checked start"},
		{"internal/relocation/walk.go", "fsmeta.OpenDir", 1,
			"opens a directory its own lstat typed one, below a checked base"},
		{"internal/sqlitefile/path.go", "osfile.Open", 1,
			"reads a database or sidecar its own lstat typed a regular file, then checks the handle " +
				"is that file"},
		{"internal/runlog/runlog.go", "osfile.OpenFile", 1,
			"creates the run's log file"},
		{"internal/users/users_windows.go", "osfile.ReadDir", 1,
			"lists the parent of a candidate the root check let through"},
		{"internal/walk/project.go", "stage.PhysicalDir", 1,
			"resolves the image root the operator gave, in NewRootCheck"},
		{"internal/walk/tree.go", "fsmeta.OpenDir", 2,
			"opens a directory its own lstat, or a listing with reparse points left out, typed one"},
	} {
		allowed[entry.file+" "+entry.call] = allowance{entry.count, entry.reason}
	}
	return allowed
}
