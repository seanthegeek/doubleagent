// Package input opens what the analyst points the analysis commands at and
// turns it into attributed artifacts. Three shapes are accepted:
//
//   - an archive `doubleagent collect` writes or any tar (plain, gzip or
//     bzip2 compressed) or zip, extracted to a new temporary directory first;
//   - an extracted collection: a directory holding manifest.jsonl, fs/ and,
//     usually, collection.json;
//   - a loose tree: any other directory, such as a copied home, a mounted
//     image or one agent directory copied on its own.
//
// With a manifest, attribution (host, user, home, agent) comes from its rows.
// Without one, homes are found by matching the catalog against the tree, the
// user is inferred from the path, and the host is left empty unless the
// caller names one. A loose tree's directories are read in name order, not
// the order the file system lists them in, so one tree gives one result on
// every file system.
//
// The paths found in the input are joined to its absolute form (absolute),
// so one input gives one set of paths however its path was spelt; a message
// names them under the input as given (Collection.ShownPath). A path built from the evidence's text (a manifest field,
// a name a parser asks Home.PathOf or Below for) is made by one function,
// below: each element must be one plain component, and each directory on
// the way down is looked at with lstat, a link (on Windows any reparse
// point) refused. os.Root would also keep a path below a directory, but it
// follows a link that stays inside and bypasses internal/osfile's Windows
// path form. A loose tree's walk enters only real directories. Extraction
// makes no link or device, checks each member name's components with
// checkElement, lstats each directory it makes and creates every file new;
// a member whose name cannot be used, or whose place holds an entry made
// for another name, is skipped with a problem naming archive and member.
package input
