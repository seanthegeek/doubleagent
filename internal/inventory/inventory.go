// Package inventory aggregates what an --inventory walk finds and writes the
// host line and the agent lines, as the sh collector's aggregate_inventory,
// write_inventory_lines and write_inventory do. collectors/docs/inventory.md
// has the output.
package inventory

import (
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/runlog"
)

// agentStats is what the walk found for one agent.
type agentStats struct {
	seen     bool
	files    int64
	bytes    int64
	first    int64
	last     int64
	evidence []string
}

// Aggregator collects the matches and files of one user's walk. It implements
// walk.Records. Use one per user, and one more for Docker volumes.
type Aggregator struct {
	order  []string
	agents map[string]*agentStats
}

// NewAggregator returns an empty Aggregator.
func NewAggregator() *Aggregator {
	return &Aggregator{agents: map[string]*agentStats{}}
}

// stats returns the entry of agent, creating it empty.
func (aggregator *Aggregator) stats(agent string) *agentStats {
	entry, found := aggregator.agents[agent]
	if !found {
		entry = &agentStats{}
		aggregator.agents[agent] = entry
	}
	return entry
}

// Match records that a catalog line of agent, with this glob, matched. An agent
// is listed in the order of its first match, and its globs are its evidence.
func (aggregator *Aggregator) Match(agent, glob string) {
	entry := aggregator.stats(agent)
	if !entry.seen {
		entry.seen = true
		entry.files, entry.bytes = 0, 0
		aggregator.order = append(aggregator.order, agent)
	}
	for _, known := range entry.evidence {
		if known == glob {
			return
		}
	}
	entry.evidence = append(entry.evidence, glob)
}

// File records one regular file of agent. A modification time of 0 or less
// counts the file but is left out of the first and last times.
func (aggregator *Aggregator) File(agent string, size, mtime int64) {
	entry := aggregator.stats(agent)
	entry.files++
	entry.bytes += size
	if mtime <= 0 {
		return
	}
	if entry.first == 0 || mtime < entry.first {
		entry.first = mtime
	}
	if entry.last == 0 || mtime > entry.last {
		entry.last = mtime
	}
}

// isoTime returns epoch seconds as UTC to the second, or "" for 0 or less.
func isoTime(epoch int64) string {
	if epoch <= 0 {
		return ""
	}
	return time.Unix(epoch, 0).UTC().Format(runlog.TimeFormat)
}

// WriteAgentLines writes one agent line for each matched agent with at least
// one file, in order of first match, for user, with the user's project count.
func (aggregator *Aggregator) WriteAgentLines(w io.Writer, host, user string, projects int) error {
	for _, agent := range aggregator.order {
		entry := aggregator.agents[agent]
		if entry.files == 0 {
			continue
		}
		line := `{"type":"agent","host":"` + manifest.EscapeJSON(host) +
			`","user":"` + manifest.EscapeJSON(user) +
			`","agent":"` + manifest.EscapeJSON(agent) +
			`","files":` + strconv.FormatInt(entry.files, 10) +
			`,"bytes":` + strconv.FormatInt(entry.bytes, 10) +
			`,"first":"` + isoTime(entry.first) + `","last":"` + isoTime(entry.last) +
			`","projects":` + strconv.Itoa(projects) +
			`,"evidence":"` + manifest.EscapeJSON(strings.Join(entry.evidence, ",")) + "\"}\n"
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	return nil
}

// Host is the content of the host line. At is the start time, UTC to the
// second; Collector is the version string.
type Host struct {
	Host            string
	Collector       string
	Mode            string
	At              string
	UsersScanned    int
	UsersUnreadable int
	DockerVolumes   int
}

// WriteHostLine writes the host line.
func WriteHostLine(w io.Writer, host Host) error {
	line := `{"type":"host","host":"` + manifest.EscapeJSON(host.Host) +
		`","collector":"` + host.Collector + `","mode":"` + host.Mode +
		`","at":"` + host.At + `","users_scanned":` + strconv.Itoa(host.UsersScanned) +
		`,"users_unreadable":` + strconv.Itoa(host.UsersUnreadable) +
		`,"docker_volumes":` + strconv.Itoa(host.DockerVolumes) + "}\n"
	_, err := io.WriteString(w, line)
	return err
}
