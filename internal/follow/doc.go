// Package follow is the one place the binary follows a link in the last
// component of a path derived from the image root: a stat, a directory or
// regular-file test, an access check, a directory listing test, a read of a
// whole file or a resolution to the physical directory. Each function takes
// the run's root check and asks it about the full path before following
// anything; when the path leads out of the image root it follows nothing
// and returns an *OutOfRootError naming the link, which the caller handles.
// A live run passes a nil check, and the functions follow as the operating
// system does.
//
// os.Stat, os.ReadFile and filepath.EvalSymlinks follow any link; os.Root
// refuses every link that leaves its directory and every absolute link, with
// a plain error. Here an absolute link naming a path under the root is
// followed, and a refusal says which link and why (*OutOfRootError), which
// the log prints.
//
// A test in this package denies by default. Outside the directories of this
// package and the ones it is built on (osfile, fsmeta, stage), every non-test
// Go file of the module may import only a reviewed standard or vendored
// package, and may use an exported name of a package that can act on a file
// path (os, os/exec, os/user, path/filepath, syscall, x/sys/unix,
// x/sys/windows and its registry, osfile, fsmeta, stage) only when a list
// says it takes no path, or never follows its last component and changes
// nothing, or a counted list gives the call's reason. The test reads
// package-level names only, so it cannot see a method or a call through an
// interface or a function value; the methods in use take a path and do not
// follow its last component (fsmeta.Opener.Open and OpenRegular open with
// O_NOFOLLOW or FILE_FLAG_OPEN_REPARSE_POINT; stage.Stager.Copy reads
// its source through them; hostcmd.Plutil runs plutil on a hint file whose
// own lstat typed it a regular file).
package follow
