package walk

import (
	"context"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// CollectUserHomes collects the catalog in every home, in order, as
// collect_user_homes does: a home that cannot be listed or entered first gets
// its error_read row, and its catalog is still tried, since a home of mode
// 711 yields the entries that need no listing.
func (collector *Collector) CollectUserHomes(ctx context.Context, homes []users.Home) error {
	for _, home := range homes {
		if home.Path == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if reason, failed := fsmeta.DirReadError(home.Path); failed {
			owner := scope{user: home.User, home: home.Path}
			if err := collector.writeReadErrors(owner, []readError{{home.Path, reason}}); err != nil {
				return err
			}
		}
		header := "User " + home.User + " (" + home.Path + ")"
		err := collector.CollectCatalog(ctx, home.User, home.Path, collector.Tables.Catalog, header)
		if err != nil {
			return err
		}
	}
	return nil
}

// WriteReadError writes the error_read row of a directory that cannot be
// listed or entered, with its lstat metadata, and its log line, as
// write_read_error_row does for a project the run collects.
func (collector *Collector) WriteReadError(user, home, path, reason string) error {
	owner := scope{user: user, home: home}
	return collector.writeReadErrors(owner, []readError{{path, reason}})
}

// catalogRun is the state of one CollectCatalog call: the header still to
// log before the first line, the stager of paths under the base, the paths
// the table's matches claim, and the symlinked directories already considered
// for a row.
type catalogRun struct {
	user   string
	base   string
	header string
	stager stage.Stager
	claims map[string]struct{}
	seen   map[string]struct{}
}

// CollectCatalog collects every match of table under base for user, logging
// header (when not empty) before the first match line, as collect_catalog
// does. The table is expanded whole first, so each match claims its path and
// a match nested in another is collected once, under its own agent.
func (collector *Collector) CollectCatalog(ctx context.Context, user, base string,
	table []catalog.Entry, header string,
) error {
	return collector.collectCatalog(ctx, user, base, table, header, collector.Stager)
}

// collectCatalog is CollectCatalog with the stager of paths under base, which
// for a project reached through a link in its home stages them where the home
// physically holds them.
func (collector *Collector) collectCatalog(ctx context.Context, user, base string,
	table []catalog.Entry, header string, stager stage.Stager,
) error {
	matches := ExpandCatalog(base, table)
	run := catalogRun{user: user, base: base, header: header, stager: stager,
		claims: map[string]struct{}{}, seen: map[string]struct{}{}}
	for _, match := range matches {
		if match.Note == "" {
			run.claims[match.Path] = struct{}{}
		}
	}
	for _, match := range matches {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := collector.collectMatch(ctx, &run, match); err != nil {
			return err
		}
	}
	return nil
}

// collectMatch logs one match and collects it, as one pass of collect_catalog's
// loop and collect_catalog_match do. A match under a symlinked directory that
// leads out of the base is not walked; that link and the in-base links before
// it get their rows. A match through in-base links is staged where the base
// physically holds it.
func (collector *Collector) collectMatch(ctx context.Context, run *catalogRun, match Match) error {
	if match.Note != "" {
		return collector.logLines(run, match.Note)
	}
	path := readField(match.Path)
	links, outside := LinkedParent(run.base, path)
	line := "  [" + match.Agent + "] " + path
	switch {
	case outside:
		line += ": under the symlinked directory " + links.Outside +
			", which leads out of the base; not walked"
		links.Passed = append(links.Passed, links.Outside)
	case collector.Inventory && match.Agent == sharedAgent:
		return nil
	case links.From != "":
		line += " (through the symlinked directory " + links.From + ", archived under " +
			physicalPath(path, links.From, links.Into) + ")"
	}
	if collector.Inventory && !outside {
		collector.Records.Match(match.Agent, match.Glob)
	}
	if err := collector.logLines(run, line); err != nil {
		return err
	}
	owner := scope{user: run.user, home: run.base, agent: match.Agent, stager: run.stager,
		claims: run.claims}
	if len(links.Passed) > 0 && !collector.Inventory {
		err := collector.recordPassedLinks(ctx, owner, run.base, links.Passed, run.seen)
		if err != nil || outside {
			return err
		}
	}
	if outside {
		return nil
	}
	owner.stager = throughLinks(run.stager, links.From, links.Into)
	_, err := collector.collectPath(ctx, owner, path)
	return err
}

// logLines logs the run's header once, before its first line, then line.
func (collector *Collector) logLines(run *catalogRun, line string) error {
	if run.header != "" {
		if err := collector.Log.Line(run.header); err != nil {
			return err
		}
		run.header = ""
	}
	return collector.Log.Line(line)
}

// CollectPath collects one path for user under home, attributed to agent, as
// collect_path does: a directory through CollectTree, an excluded path as one
// skipped_excluded row, anything else through the per-file staging. A
// directory that cannot be listed or entered gets one error_read row here and
// is not walked outside --inventory. It returns how many directories the
// walk could not read.
func (collector *Collector) CollectPath(ctx context.Context, user, home, agent, path string,
) (int, error) {
	owner := scope{user: user, home: home, agent: agent, stager: collector.Stager}
	return collector.collectPath(ctx, owner, path)
}

// collectPath is CollectPath for one scope.
func (collector *Collector) collectPath(ctx context.Context, owner scope, path string,
) (int, error) {
	meta, statError := fsmeta.Lstat(path)
	if statError == nil && meta.Kind == fsmeta.Dir {
		if reason, failed := fsmeta.DirReadError(path); failed && !collector.Inventory {
			return 0, collector.writeReadErrors(owner, []readError{{path, reason}})
		}
		return collector.collectTree(ctx, owner, path)
	}
	if matchesAny(collector.excludePatterns(owner.home), path) {
		if collector.Inventory {
			return 0, nil
		}
		return 0, collector.writeExcluded(ctx, owner, []string{path})
	}
	if collector.Inventory {
		if statError == nil && meta.Kind == fsmeta.File {
			collector.Records.File(owner.agent, meta.Size, meta.Mtime)
		}
		return 0, nil
	}
	return 0, collector.stageFiles(owner, []string{path}, collector.Secret)
}

// StageFile stages one path for user under home, attributed to agent, staged
// by stager, as the relocation step runs the staging worker for a file that
// names a relocation variable. With forceSecret the file is flagged secret
// whatever its name, so --no-secrets skips it.
func (collector *Collector) StageFile(user, home, agent string, stager stage.Stager, path string,
	forceSecret bool,
) error {
	secret := collector.Secret
	if forceSecret {
		secret = func(string) bool { return true }
	}
	owner := scope{user: user, home: home, agent: agent, stager: stager}
	return collector.stageFiles(owner, []string{path}, secret)
}
