// Package stage puts collected files into the staging directory, the tree that
// becomes the archive: the staged path of each file, the copy and its hash,
// recreated symlinks, and the removal of a failed partial copy.
//
// Every staged write, listing and removal goes through the open staging
// directory (Tree), never through what its name points at now, so an
// account that moves the directory aside during the run and puts a link in
// its place changes nothing the run writes or the members it names: a
// member is spelled from the name it was created by, never from where the
// tree is or was (member). A recreated link in the
// staging tree may lead to the source or to the analyst's own files, so
// the stager lists or opens nothing below a link: a write, an open or a
// link's creation first passes requireInside for its directory, and every
// respelling walks the tree with walkStaged, which stops at a link.
package stage
