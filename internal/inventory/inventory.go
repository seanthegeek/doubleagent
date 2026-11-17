// Package inventory aggregates what an --inventory walk finds and writes the
// host line and the agent lines, one JSON object each through encoding/json,
// as the sh collector's aggregate_inventory, write_inventory_lines and
// write_inventory do. collectors/docs/inventory.md has the output.
package inventory

import (
	"encoding/json"
	"io"
	"slices"
	"strings"
	"time"

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
	if !slices.Contains(entry.evidence, glob) {
		entry.evidence = append(entry.evidence, glob)
	}
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

// agentLine is the JSON form of an agent line. Every field is written, an
// empty first or last time as "".
type agentLine struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	User     string `json:"user"`
	Agent    string `json:"agent"`
	Files    int64  `json:"files"`
	Bytes    int64  `json:"bytes"`
	First    string `json:"first"`
	Last     string `json:"last"`
	Projects int    `json:"projects"`
	Evidence string `json:"evidence"`
}

// hostLine is the JSON form of the host line.
type hostLine struct {
	Type            string `json:"type"`
	Host            string `json:"host"`
	Collector       string `json:"collector"`
	Mode            string `json:"mode"`
	Time            string `json:"at"`
	UsersScanned    int    `json:"users_scanned"`
	UsersUnreadable int    `json:"users_unreadable"`
	DockerVolumes   int    `json:"docker_volumes"`
}

// lineEncoder returns an encoder that writes each value as one compact line
// to w, with <, > and & as they are.
func lineEncoder(w io.Writer) *json.Encoder {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder
}

// WriteAgentLines writes one agent line for each matched agent with at least
// one file, in order of first match, for user, with the user's project count.
func (aggregator *Aggregator) WriteAgentLines(w io.Writer, host, user string, projects int) error {
	encoder := lineEncoder(w)
	for _, agent := range aggregator.order {
		entry := aggregator.agents[agent]
		if entry.files == 0 {
			continue
		}
		err := encoder.Encode(agentLine{
			Type: "agent", Host: host, User: user, Agent: agent, Files: entry.files,
			Bytes: entry.bytes, First: isoTime(entry.first), Last: isoTime(entry.last),
			Projects: projects, Evidence: strings.Join(entry.evidence, ","),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Host is the content of the host line. Timestamp is the start time, UTC to the
// second; Collector is the version string.
type Host struct {
	Host            string
	Collector       string
	Mode            string
	Timestamp       string
	UsersScanned    int
	UsersUnreadable int
	DockerVolumes   int
}

// WriteHostLine writes the host line.
func WriteHostLine(w io.Writer, host Host) error {
	return lineEncoder(w).Encode(hostLine{
		Type: "host", Host: host.Host, Collector: host.Collector, Mode: host.Mode,
		Time: host.Timestamp, UsersScanned: host.UsersScanned,
		UsersUnreadable: host.UsersUnreadable, DockerVolumes: host.DockerVolumes,
	})
}
