package walk

import (
	"context"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
)

// CollectProject collects the project catalog in project for user, as
// collect_projects does for one project; home is the user's home holding the
// project, or "". A project whose path runs through a symlinked directory in
// home is handled as a catalog match is: a link out of home leaves the
// project unwalked; links inside get their rows and the files keep their
// recorded path but are staged where home physically holds them. The sh
// collector stages them through the recreated link instead: a deviation.
func (collector *Collector) CollectProject(ctx context.Context, user, home, project string) error {
	var links Links
	outside := false
	if home != "" {
		links, outside = LinkedParent(home, project)
	}
	if err := collector.Log.Line(projectLine(user, project, links, outside)); err != nil {
		return err
	}
	if outside {
		links.Passed = append(links.Passed, links.Outside)
	} else if reason, failed := fsmeta.DirReadError(project); failed {
		if err := collector.WriteReadError(user, project, project, reason); err != nil {
			return err
		}
	}
	if len(links.Passed) > 0 && !collector.Inventory {
		owner := scope{user: user, home: home, agent: sharedAgent, stager: collector.Stager}
		err := collector.recordPassedLinks(ctx, owner, home, links.Passed, map[string]struct{}{})
		if err != nil {
			return err
		}
	}
	if outside {
		return nil
	}
	stager := collector.Stager.Through(links.From, links.Into)
	return collector.collectCatalog(ctx, user, project, collector.Tables.ProjectCatalog, "", stager)
}

// projectLine returns the log line that opens a project, with the words a
// catalog match's line ends in when its path runs through a symlinked
// directory of its home.
func projectLine(user, project string, links Links, outside bool) string {
	line := "Project " + project
	if user != "" {
		line = "Project [" + user + "] " + project
	}
	switch {
	case outside:
		return line + ": under the symlinked directory " + links.Outside +
			", which leads out of the base; not walked"
	case links.From != "":
		return line + " (through the symlinked directory " + links.From + ", archived under " +
			physicalPath(project, links.From, links.Into) + ")"
	}
	return line
}
