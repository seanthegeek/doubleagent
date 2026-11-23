// Package evidencepath reads a path as the evidence's machine wrote it: a
// Windows path recorded on a Windows host and analysed on a Mac, or a POSIX
// path analysed on Windows. path/filepath follows the rules of the host the
// binary runs on, so it cannot read these; path reads slash forms only. The
// few rules here (a drive letter, either slash as a Windows separator) are
// the ones both the analysis and the project discovery of `collect` need.
package evidencepath
