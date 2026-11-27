package collect_test

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/glob"
)

// coverageLine is one table line, how to find its evidence in the runs and
// how to find a fixture for it in an image.
type coverageLine struct {
	table, line string
	evidence    func(runs []coverageRun) bool
	fixture     func(image imageFixtures) bool
}

// key names the line in testdata/coverage-uncovered.txt and in the test's
// subtests: its table, a space and the line.
func (line coverageLine) key() string {
	return line.table + " " + line.line
}

// anyRow reports whether keep holds of a row of a run.
func anyRow(runs []coverageRun, keep func(coverageRun, manifestRow) bool) bool {
	for _, run := range runs {
		for _, row := range run.rows {
			if keep(run, row) {
				return true
			}
		}
	}
	return false
}

// matchesCatalogGlob reports whether rel, a path below a base, lies at or
// below a match of a catalog glob, whose segments each match one name.
func matchesCatalogGlob(pattern, rel string) bool {
	segments, names := strings.Split(pattern, "/"), strings.Split(rel, "/")
	if rel == "" || len(names) < len(segments) {
		return false
	}
	for index, segment := range segments {
		if !glob.MatchSegment(segment, names[index]) {
			return false
		}
	}
	return true
}

// anySuffix reports whether matches holds of the end of an entry's path
// below a root, from one of its names on.
func (image imageFixtures) anySuffix(matches func(rel string) bool) bool {
	for _, entry := range image.entries {
		for _, root := range image.roots {
			suffix, found := strings.CutPrefix(entry.path, root+"/")
			for found {
				if matches(suffix) {
					return true
				}
				_, suffix, found = strings.Cut(suffix, "/")
			}
		}
	}
	return false
}

// anyBelow reports whether matches holds of the path of an entry below one
// of the directories, relative to that directory.
func (image imageFixtures) anyBelow(directories []string, matches func(rel string) bool) bool {
	for _, entry := range image.entries {
		for _, base := range directories {
			if rel, found := strings.CutPrefix(entry.path, base+"/"); found && matches(rel) {
				return true
			}
		}
	}
	return false
}

// anyBelowBase reports whether matches holds of the path of an entry below
// one of the bases, relative to that base.
func (image imageFixtures) anyBelowBase(matches func(rel string) bool) bool {
	if image.anywhere {
		return image.anySuffix(matches)
	}
	return image.anyBelow(image.bases, matches)
}

// artifactLine is the coverage line of a CATALOG or PROJECT_CATALOG entry:
// a collected row of its agent at or below a path the glob matches.
func artifactLine(table string, entry catalog.Entry, fold func(string) string) coverageLine {
	pattern := fold(entry.Glob)
	return coverageLine{table: table, line: entry.Agent + "|" + entry.Glob,
		evidence: func(runs []coverageRun) bool {
			return anyRow(runs, func(_ coverageRun, row manifestRow) bool {
				return row.status == "collected" && row.agent == entry.Agent && matchesCatalogGlob(pattern, row.rel)
			})
		},
		fixture: func(image imageFixtures) bool {
			return image.anyBelowBase(func(rel string) bool { return matchesCatalogGlob(pattern, rel) })
		}}
}

// isVolumeData reports whether base, a row's home, is a volume's _data
// directory.
func isVolumeData(base string) bool {
	return filepath.Base(base) == "_data"
}

// anyBelowVolume is anyBelowBase for a volume| line: only a volume's _data
// directory is a base.
func (image imageFixtures) anyBelowVolume(matches func(rel string) bool) bool {
	if image.anywhere {
		return image.anySuffix(func(rel string) bool {
			rest, found := strings.CutPrefix(rel, "_data/")
			return found && matches(rest)
		})
	}
	var volumes []string
	for _, base := range image.bases {
		if base == "_data" || strings.HasSuffix(base, "/_data") {
			volumes = append(volumes, base)
		}
	}
	return image.anyBelow(volumes, matches)
}

// pathLine is the coverage line of a glob matched against a whole path
// below a base: a row that keep picks whose path the glob matches.
func pathLine(table, line string, fold func(string) string, keep func(coverageRun, manifestRow) bool) coverageLine {
	text, volume := strings.CutPrefix(line, "volume|")
	pattern := fold(text)
	return coverageLine{table: table, line: line,
		evidence: func(runs []coverageRun) bool {
			return anyRow(runs, func(run coverageRun, row manifestRow) bool {
				return keep(run, row) && row.rel != "" && glob.MatchPath(pattern, row.rel) &&
					(!volume || isVolumeData(row.home))
			})
		},
		fixture: func(image imageFixtures) bool {
			matches := func(rel string) bool { return glob.MatchPath(pattern, rel) }
			if volume {
				return image.anyBelowVolume(matches)
			}
			return image.anyBelowBase(matches)
		}}
}

// volumePrefixed returns the globs of the volume| lines of an EXCLUDES or
// SECRET_GLOBS table as the catalog file writes them.
func volumePrefixed(globs []string) []string {
	lines := make([]string, len(globs))
	for i, text := range globs {
		lines[i] = "volume|" + text
	}
	return lines
}

// imageFixtures is an image's entries below its work directory, the roots
// runs name there, and the bases a run collects from (homes, projects and
// volume data directories), each a path below the work directory with "/"
// between names; with anywhere set, every directory below a root is a
// base.
type imageFixtures struct {
	entries  []imageEntry
	roots    []string
	bases    []string
	anywhere bool
}

// volumeLine is the coverage line of a DOCKER_VOLUMES entry: a row of its
// agent collected from a volume whose name the glob matches.
func volumeLine(entry catalog.Entry, fold func(string) string) coverageLine {
	pattern := fold(entry.Glob)
	return coverageLine{table: "DOCKER_VOLUMES", line: entry.Agent + "|" + entry.Glob,
		evidence: func(runs []coverageRun) bool {
			return anyRow(runs, func(_ coverageRun, row manifestRow) bool {
				volume, isVolume := strings.CutSuffix(row.home, string(filepath.Separator)+"_data")
				return isVolume && row.agent == entry.Agent && glob.MatchPath(pattern, fold(filepath.Base(volume)))
			})
		},
		fixture: func(image imageFixtures) bool {
			for _, item := range image.entries {
				names := strings.Split(item.path, "/")
				for index := 1; index+1 < len(names); index++ {
					if names[index-1] == "volumes" && names[index+1] == "_data" && glob.MatchPath(pattern, names[index]) {
						return true
					}
				}
			}
			return false
		}}
}

// relocationLine is the coverage line of a RELOCATION_VARS entry: a hint
// naming its variable for its agent.
func relocationLine(variable catalog.RelocationVar) coverageLine {
	return coverageLine{table: "RELOCATION_VARS",
		line: variable.Agent + "|" + variable.Variable + "|" + variable.Replaces,
		evidence: func(runs []coverageRun) bool {
			for _, run := range runs {
				for _, hint := range run.hints {
					if hint.variable == variable.Variable && hint.agent == variable.Agent {
						return true
					}
				}
			}
			return false
		},
		fixture: func(image imageFixtures) bool {
			for _, entry := range image.entries {
				if strings.Contains(entry.text, variable.Variable) {
					return true
				}
			}
			return false
		}}
}

// startupLine is the coverage line of a STARTUP_FILES entry: a hint read
// from a file the glob matches below a base of its scope, a home, a
// project or the root.
func startupLine(startup catalog.StartupFile, fold func(string) string) coverageLine {
	pattern := fold(strings.TrimPrefix(startup.Glob, "/"))
	return coverageLine{table: "STARTUP_FILES", line: startup.Scope + "|" + startup.Glob,
		evidence: func(runs []coverageRun) bool {
			for _, run := range runs {
				bases := map[string][]string{"home": run.homes, "project": run.projects, "system": {run.root}}
				for _, hint := range run.hints {
					for _, base := range bases[startup.Scope] {
						if rel := fold(relativeTo(base, hint.source)); rel != "" && glob.MatchPath(pattern, rel) {
							return true
						}
					}
				}
			}
			return false
		},
		fixture: func(image imageFixtures) bool {
			matches := func(rel string) bool { return glob.MatchPath(pattern, rel) }
			if startup.Scope == "system" {
				return image.anyBelow(image.roots, matches)
			}
			return image.anyBelowBase(matches)
		}}
}

// coverageLines returns one coverage line for each table line, two for a
// SECRET_GLOBS line. The evidence: for a CATALOG or PROJECT_CATALOG line a
// collected row of its agent on a path it matches; EXCLUDES a
// skipped_excluded row; SECRET_GLOBS a secret row of a default run and a
// skipped_secret row under --no-secrets; STARTUP_FILES a hint read from a
// file it matches; RELOCATION_VARS a hint naming its variable for its
// agent; DOCKER_VOLUMES a row of its agent in a volume it names. Globs are
// matched after fold, which the paths have been through too.
func coverageLines(tables *catalog.Tables, fold func(string) string) []coverageLine {
	var lines []coverageLine
	for _, entry := range tables.Catalog {
		lines = append(lines, artifactLine("CATALOG", entry, fold))
	}
	for _, entry := range tables.ProjectCatalog {
		lines = append(lines, artifactLine("PROJECT_CATALOG", entry, fold))
	}
	for _, pattern := range slices.Concat(tables.Excludes, volumePrefixed(tables.VolumeExcludes)) {
		lines = append(lines, pathLine("EXCLUDES", pattern, fold, func(run coverageRun, row manifestRow) bool {
			return run.option == "" && row.status == "skipped_excluded"
		}))
	}
	for _, pattern := range slices.Concat(tables.SecretGlobs, volumePrefixed(tables.VolumeSecretGlobs)) {
		lines = append(lines, pathLine("SECRET_GLOBS", pattern, fold, func(run coverageRun, row manifestRow) bool {
			return run.option == "" && row.secret
		}), pathLine("SECRET_GLOBS --no-secrets", pattern, fold, func(run coverageRun, row manifestRow) bool {
			return run.option == "--no-secrets" && row.status == "skipped_secret"
		}))
	}
	for _, entry := range tables.DockerVolumes {
		lines = append(lines, volumeLine(entry, fold))
	}
	for _, variable := range tables.RelocationVars {
		lines = append(lines, relocationLine(variable))
	}
	for _, startup := range tables.StartupFiles {
		lines = append(lines, startupLine(startup, fold))
	}
	return lines
}

// folded returns the fixtures with every path through fold.
func (image imageFixtures) folded(fold func(string) string) imageFixtures {
	result := imageFixtures{anywhere: image.anywhere}
	for _, entry := range image.entries {
		result.entries = append(result.entries, imageEntry{path: fold(entry.path), text: entry.text, link: entry.link})
	}
	for _, root := range image.roots {
		result.roots = append(result.roots, fold(root))
	}
	for _, base := range image.bases {
		result.bases = append(result.bases, fold(base))
	}
	return result
}
