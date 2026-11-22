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
// plugin, net with its Unix sockets, io/ioutil) are not listed, so importing
// one fails.
func reviewedImports() map[string]string {
	noPath := []string{"archive/tar", "bufio", "bytes", "cmp", "compress/bzip2", "compress/flate",
		"compress/gzip", "context", "crypto/rand", "crypto/sha256", "embed", "encoding/base64",
		"encoding/binary", "encoding/hex",
		"errors", "fmt", "hash", "hash/crc32", "io", "io/fs", "iter", "math", "math/big", "os/signal",
		"path", "regexp", "runtime", "runtime/debug", "slices", "sort", "strconv", "strings",
		"testing", "time", "unicode", "unicode/utf16", "unicode/utf8",
		// Reads a database's schema once however many goroutines ask for
		// it (sqlitefile); the name used is Once, which takes no path.
		"sync",
		// Multiplies two 64-bit numbers into 128 bits for the text digests
		// of deduplication (dedup/digest.go); the name used is Mul64, pure
		// arithmetic that takes no path.
		"math/bits",
		// Hashes each prefix of a project path as it is read, one byte at
		// a time, to look the homes holding it up (analyze/projects.go);
		// the names used are Hash, its SetSeed, WriteByte and Sum64,
		// MakeSeed, Seed and String, none of which takes a path.
		"hash/maphash",
		// Decodes zstd data from an io.Reader the caller gives it; the names
		// used are NewReader, WithDecoderConcurrency, WithDecoderLowmem,
		// WithDecoderMaxWindow, ErrWindowSizeExceeded and ErrDecoderSizeExceeded,
		// none of which takes a path.
		"github.com/klauspost/compress/zstd",
		// Reads an XML document from an io.Reader over bytes already read
		// through the parser kit (Tabby's IntelliJ history, Cody's chat
		// history); the names used are NewDecoder, the token types, Name and
		// SyntaxError.
		"encoding/xml",
		// Writes the manifest rows, collection.json, the inventory lines and
		// the analysis outputs to an io.Writer the caller gives it, and reads
		// JSON from text or bytes already read; the names used are NewEncoder
		// and its SetEscapeHTML, SetIndent and Encode, NewDecoder and its
		// UseNumber, Token and InputOffset, and the Delim, Number,
		// SyntaxError and Token types, none of which takes a path.
		"encoding/json",
		// Copies JSON outputs with each string through the replacement of
		// withheld secrets (analyze/withheld.go: ValueOf, New, MakeSlice,
		// MakeMapWithSize; Value's Kind, String, Convert, IsNil, Elem, Set,
		// Type, Len, Index, Interface, NumField, Field, MapRange and
		// SetMapIndex; MapIter's Next, Key and Value; Type's Elem, Field and
		// Kind; StructField's IsExported, Anonymous, Type and Tag.Get; the
		// Kind constants String, Pointer, Interface, Slice, Array, Map and
		// Struct), and compares a parser test's two modes' sessions
		// (parser/parsertest/parsertest.go, imported only by tests:
		// DeepEqual); all work on values in memory and take no path.
		"reflect",
		// Writes the fleet CSV's records to an io.Writer the caller gives
		// it; the names used are NewWriter and the Writer's WriteAll, which
		// take no path.
		"encoding/csv",
		// Parses URLs from evidence text into their parts: a file: URI
		// (evidencepath.FromFileURI), a repository or server address
		// (cursor, qwencode, tabby) and a percent-escaped project name
		// (geminicli); the names used are Parse and PathUnescape, with the
		// parsed URL's fields and Hostname and EscapedPath methods, which
		// work on text and take no path.
		"net/url",
		// Reads a mail date from evidence text (Cody's chat ids); the one
		// name used is ParseDate, which works on text and takes no path.
		"net/mail",
		// Decodes the character references of evidence text (a Crush channel
		// message's attribute values); the name used is UnescapeString,
		// which works on text and takes no path.
		"html",
		// Reads the command line of each subcommand from the argument list
		// it is given; the names used are NewFlagSet, ContinueOnError and the
		// FlagSet type with its SetOutput (always io.Discard), StringVar,
		// BoolVar, Func, Parse, Name, Arg, Args and NArg, none of which takes
		// a path.
		"flag"}
	guarded := []string{"archive/zip", "os", "os/exec", "os/user", "path/filepath", "syscall",
		"golang.org/x/sys/unix", "golang.org/x/sys/windows", "golang.org/x/sys/windows/registry",
		"github.com/BurntSushi/toml"}
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
// functions it wraps, the staging layer, which works on the output
// directory and resolves the root the run is given, and imagetest, which
// only tests import (TestTheBinaryLinksNoTestHelper) and which builds
// fixture trees under a test's temporary directory.
func ownerPackages() []string {
	return []string{"internal/follow", "internal/osfile", "internal/fsmeta", "internal/stage",
		"internal/collect/imagetest"}
}

// safeNames are, by import path, the exported names of guarded packages that
// take no file-system path or work on one as text alone: constants, types,
// error values, and functions such as os.Getenv or filepath.Join. A name that
// takes a path belongs on noFollowNames or allowedCalls instead.
func safeNames() map[string][]string {
	return map[string][]string{
		// The process, its environment, standard streams and signal type,
		// the null device's name, file types and flags, and two error types. Getwd
		// takes no path; the operating system answers it from the process,
		// though the Unix fallback stats "." and the working directory to
		// check $PWD. SameFile compares two FileInfo values already read.
		"os": {"Args", "DevNull", "DirEntry", "Environ", "Exit", "File", "FileInfo", "Getenv", "Getuid",
			"Getwd", "Hostname", "Interrupt", "LinkError", "ModeSymlink", "O_CREATE", "O_EXCL",
			"O_TRUNC", "O_WRONLY", "SameFile", "Signal", "Stderr", "Stdout", "SyscallError", "TempDir"},
		// A zip reader over a handle already open, its types and errors;
		// OpenReader, which opens a path by name, is left out.
		"archive/zip": {"ErrAlgorithm", "ErrChecksum", "ErrFormat", "ErrInsecurePath", "File", "FileHeader",
			"NewReader"},
		// The running account's own record, looked up by the system.
		"os/user": {"Current"},
		// Text operations on a path; none touches the file system.
		// A parser package may use none of the names that build a path:
		// TestNoParserPackageBuildsAPath.
		"path/filepath": {"Base", "Clean", "Dir", "IsAbs", "Join", "Separator", "SplitList", "ToSlash",
			"VolumeName"},
		// Constants, three signals, an attribute record type, and error numbers
		// with their type, which internal/reason and internal/input compare
		// errors against.
		"syscall": {"EILSEQ", "EISDIR", "ELOOP", "ENAMETOOLONG", "ENOTDIR", "ERROR_MORE_DATA", "Errno",
			"FILE_ATTRIBUTE_DIRECTORY", "FILE_ATTRIBUTE_REPARSE_POINT", "SIGHUP", "SIGPIPE",
			"SIGTERM", "Win32FileAttributeData"},
		// Constants, the umask, the kernel's name and a byte conversion.
		"golang.org/x/sys/unix": {"AT_EACCESS", "AT_FDCWD", "ByteSliceToString", "Umask", "Uname",
			"Utsname", "W_OK", "X_OK"},
		// Security identifiers, the version call, a token type, constants and
		// string conversions.
		"golang.org/x/sys/windows": {"CreateWellKnownSid", "ERROR_INVALID_NAME",
			"FILE_ATTRIBUTE_DIRECTORY", "MAX_LONG_PATH", "RtlGetVersion", "Token",
			"UTF16PtrFromString", "UTF16ToString", "WinBuiltinAdministratorsSid"},
		// Decode reads TOML text it is given, and the other two are an error
		// type and a key path type. DecodeFile and DecodeFS open paths and are
		// not listed.
		"github.com/BurntSushi/toml": {"Decode", "Key", "ParseError"},
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
		// The staging types and a text comparison of two paths. A Tree's
		// methods take names below the staging directory it holds open.
		"github.com/seanthegeek/doubleagent/internal/stage": {"Stager", "Tree", "Under",
			"NewPlaces", "IsCollision"},
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
			"removes the partial archive it created when the write fails"},
		{"internal/collect/run_finish.go", "osfile.Remove", 4,
			"removes what holds the run's archive name and partial name before the archive is " +
				"written, never following a link, and the run's own partial archive and checksum " +
				"file when the run stops or fails before its archive is moved"},
		{"internal/collect/run_finish.go", "osfile.Rename", 1,
			"moves the run's own finished archive from its partial name to its name"},
		{"internal/collect/run.go", "osfile.Remove", 1,
			"removes the run's own collection.json when the run is interrupted"},
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
		{"internal/archive/archive.go", "osfile.OpenFile", 2,
			"creates the partial archive and its checksum file in the output directory new, failing " +
				"if anything, a link included, holds the name"},
		{"internal/collect/run_setup.go", "osfile.MkdirAll", 1,
			"creates the output directory -o names"},
		{"internal/collect/run_setup.go", "stage.CreateTree", 1,
			"creates the staging directory new, failing if anything, a link included, holds the " +
				"name, and holds it open; every later staged write goes through that tree"},
		{"internal/collect/run_setup.go", "osfile.OpenRoot", 1,
			"opens the output directory once to create and remove the probe file below it"},
		{"internal/collect/run_setup.go", "stage.MarkCaseSensitive", 1,
			"marks the run's own new staging fs directory case-sensitive"},
		{"internal/shortname/shortname_windows.go", "windows.GetShortPathName", 1,
			"names a test directory the helper made under the test's temporary directory"},
		{"internal/shortname/shortname_windows.go", "windows.GetLongPathName", 1,
			"names a test directory the helper made under the test's temporary directory"},
		{"internal/shortname/shortname_windows.go", "osfile.MkdirAll", 1,
			"makes a test directory under the test's own temporary directory"},
		{"internal/collect/options.go", "osfile.Stat", 1,
			"tests -r and -o as the operator gives them"},
		{"internal/collect/run_finish.go", "osfile.Open", 1,
			"copies the run's own output files"},
		{"internal/collect/run_finish.go", "osfile.OpenFile", 1,
			"creates each of the run's own output files new, failing if anything, a link included, " +
				"holds the name"},
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
		{"internal/parser/files.go", "osfile.ReadDir", 1,
			"lists a directory of the input its own lstat typed a directory, not a link, at a path " +
				"the kit built (Beside, Above, Under, Home.PathOf, or a Directory's Path or Child), " +
				"which refused a link on the way down"},
		{"internal/sqlitefile/path.go", "osfile.Open", 1,
			"reads a database or sidecar its own lstat typed a regular file, then checks the handle " +
				"is that file"},
		{"internal/runlog/runlog.go", "osfile.OpenFile", 1,
			"creates the run's log file"},
		{"internal/users/users_windows.go", "osfile.ReadDir", 1,
			"lists the parent of a candidate the root check let through"},
		{"internal/walk/project.go", "stage.PhysicalDir", 1,
			"resolves the image root the operator gave, in NewRootCheck"},
		{"internal/input/input.go", "osfile.Stat", 1,
			"stats the input the analyst names, following a link as the analyst chose it"},
		{"internal/input/input.go", "osfile.ReadDir", 2,
			"lists the input directory the analyst names, to refuse an unlistable one, and is the " +
				"loose walk's lister: a home, the input, or a directory its parent's listing typed a real directory"},
		{"internal/input/input.go", "osfile.RemoveAll", 1,
			"removes the extraction directory the run made, after its lstat typed it one"},
		{"internal/input/archive.go", "osfile.Open", 2,
			"reads the archive the analyst names, to fail before extracting and to hash it"},
		{"internal/input/archive.go", "osfile.Stat", 1,
			"tests the checksum file beside the archive the analyst names"},
		{"internal/input/archive.go", "osfile.ReadFile", 1,
			"reads the checksum file beside the archive the analyst names"},
		{"internal/input/archive.go", "osfile.MkdirAll", 1,
			"creates the analyst's work directory"},
		{"internal/input/archive.go", "osfile.MkdirTemp", 1,
			"creates a new extraction directory, under a name nothing else holds, in the " +
				"analyst's work directory or the system's temporary directory"},
		{"internal/input/extract.go", "osfile.Open", 1,
			"reads the archive the analyst names"},
		{"internal/input/extract.go", "osfile.MkdirAll", 1,
			"creates one directory of the extraction whose lstat showed nothing there"},
		{"internal/input/extract.go", "osfile.OpenFile", 2,
			"creates an extracted file exclusively, so never through a link"},
		{"internal/input/extract.go", "osfile.Remove", 1,
			"removes a regular file an earlier member of the same archive wrote"},
		{"internal/input/extract.go", "osfile.Chtimes", 2,
			"sets the time of a file or directory the extraction made, from its member"},
		{"internal/input/loose.go", "stage.PhysicalDir", 1,
			"resolves the input directory the analyst names, for the owner of its parent"},
		{"internal/analyze/fleet.go", "osfile.MkdirAll", 1,
			"creates the directory of the CSV the analyst names with -o"},
		{"internal/analyze/fleet.go", "osfile.Rename", 1,
			"moves the temporary file it created new beside the CSV the analyst names with -o onto " +
				"that name, replacing an existing file there rather than writing into it"},
		{"internal/analyze/fleet.go", "osfile.Remove", 1,
			"removes the temporary file it created new beside the CSV when writing or moving it fails"},
		{"internal/analyze/timeline.go", "osfile.ReadDir", 1,
			"lists the output directory the analyst names, which its lstat showed a directory, " +
				"to refuse one that is not empty"},
		{"internal/analyze/output.go", "osfile.OpenFile", 1,
			"creates a file new, failing if anything, a link included, holds its name: timeline's " +
				"probe file under a random name and each output file in its output directory, and " +
				"fleet's temporary file under a random name beside the CSV"},
		{"internal/analyze/timeline.go", "osfile.Remove", 1,
			"removes the directories that timeline, fleet, or the work directory of detect and " +
				"timeline created, once the run leaves them empty"},
		{"internal/analyze/timeline.go", "osfile.OpenRoot", 1,
			"opens the output directory once to create and remove the probe file below it"},
		{"internal/analyze/timeline.go", "osfile.MkdirAll", 1,
			"creates the output directory the analyst names as OUTPUT_DIR"},
		{"internal/analyze/outside_unix.go", "filepath.EvalSymlinks", 1,
			"resolves a link on the output path the analyst typed for timeline or fleet, to tell " +
				"whether it lies inside an input directory: what it gives is only lstat'ed and " +
				"compared with os.SameFile, never opened, read or written"},
		{"internal/input/paths_windows.go", "filepath.Abs", 1,
			"makes absolute, as Windows itself places it, a path the analyst typed that has no drive " +
				"and root and is not a UNC path: an analysis command's input, which is read there, " +
				"the work directory an archive is extracted under, made there, and the output of " +
				"timeline or fleet, written there, whose place the outside-the-input check lstats " +
				"and compares with the input's entry through os.SameFile, which on Windows opens a " +
				"handle with no access rights to identify a file (loadFileId in os/types_windows.go), " +
				"nothing being read or written through it"},
		{"internal/walk/tree.go", "fsmeta.OpenDir", 2,
			"opens a directory its own lstat, or a listing with reparse points left out, typed one"},
	} {
		allowed[entry.file+" "+entry.call] = allowance{entry.count, entry.reason}
	}
	return allowed
}
