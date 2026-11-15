// Package summary writes the run's two reports: collection.json, the
// machine-readable record of the run, and the stdout summary, in the layout of
// the sh collector's write_summary and print_result.
package summary

import (
	"io"

	"github.com/seanthegeek/doubleagent/internal/manifest"
	"github.com/seanthegeek/doubleagent/internal/users"
	"github.com/seanthegeek/doubleagent/internal/volumes"
)

// Tool is the value of collection.json's tool field.
const Tool = "doubleagent"

// Options is the part of the command line collection.json records.
type Options struct {
	Full           bool
	NoSecrets      bool
	MaxFileBytes   int64
	UsersFilter    string
	NoDocker       bool
	NoProjects     bool
	Projects       []string
	EnvScanHome    bool
	EnvScanMaxSize int64
}

// Hint is one relocation hint as collection.json lists it.
type Hint struct {
	User     string
	Source   string
	Line     int
	Variable string
	Agent    string
	Value    string
}

// Relocation is the relocation search's result. RegistryReason is the text of
// the registry object's reason field; its lists are always empty, since only
// the PowerShell collector reads a registry.
type Relocation struct {
	Hints            []Hint
	RegistryRead     bool
	RegistryReason   string
	FilesSearched    int
	FilesSkippedSize int
	Resolved         int
}

// Summary is everything collection.json and the stdout summary report.
// Started and Finished are UTC times already formatted. Users, Homes and
// ActiveUsers are parallel lists of the enumerated homes and of those that gave
// a row; Projects holds project directories only.
type Summary struct {
	Version     string
	Hostname    string
	Mode        string
	Root        string
	Platform    string
	UID         int
	Started     string
	Finished    string
	Options     Options
	Plutil      bool
	Users       []string
	Homes       []string
	ActiveUsers []string
	Projects    []string
	Counts      manifest.Counts
	Docker      volumes.Counts
	Relocation  Relocation
	Notes       []string
	Archive     string

	// The fields below feed only the stdout summary.
	ArchivePath  string
	ArchiveSize  int64
	ArchiveSHA   string
	ManifestPath string
	SummaryPath  string
	LogPath      string
}

// ActiveUsers returns the homes that have a manifest row other than their own
// unreadable-directory row, in order, as list_active_users does. withRows is
// manifest.Writer.HomesWithRows.
func ActiveUsers(homes []users.Home, withRows map[string]struct{}) []users.Home {
	var active []users.Home
	for _, home := range homes {
		if _, found := withRows[home.Path]; home.Path != "" && found {
			active = append(active, home)
		}
	}
	return active
}

// WriteCollectionJSON writes collection.json, in the sh collector's line
// breaks and spacing.
func WriteCollectionJSON(w io.Writer, summary Summary) error {
	_, err := w.Write(collectionJSON(summary))
	return err
}
