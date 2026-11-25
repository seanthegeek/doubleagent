// Package stage puts collected files into the staging directory, the tree that
// becomes the archive: the staged path of each file, the copy and its hash,
// recreated symlinks, and the removal of a failed partial copy.
//
// Every staged write, listing and removal goes through the open staging
// directory (Tree), never by its name, so moving the directory aside during
// the run and putting a link in its place changes nothing the run writes or
// the members it names (member). A link recreated in the tree may lead to
// the source or to the analyst's own files, so nothing below a link is
// listed or opened: every write, open or link creation first passes
// findInside for its directory, and every respelling walks the tree with
// walkStaged, which stops at a link.
package stage
