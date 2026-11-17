// Package summary writes the run's two reports: collection.json, the
// machine-readable record of the run, written through encoding/json, and the
// stdout summary, in the layout of the sh collector's print_result.
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
	// NoPreserveAtime records --no-preserve-atime, written on every
	// platform though it changes the copy on Windows alone.
	NoPreserveAtime bool
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

// Relocation is the relocation search's result. RegistryRead and
// RegistryReason are the registry object's read and reason fields, and
// UsersRead, UsersNotLoaded and UsersDenied its three user lists: the users
// whose hive the registry pass read, found not loaded, or could not read. The
// lists stay empty off a live Windows host.
type Relocation struct {
	Hints            []Hint
	RegistryRead     bool
	RegistryReason   string
	UsersRead        []string
	UsersNotLoaded   []string
	UsersDenied      []string
	FilesSearched    int
	FilesSkippedSize int
	Resolved         int
}

// Identity is the account the run ran as. UID is the effective user ID
// collection.json records on Unix; RunAs (DOMAIN\user) and Admin, whether
// the account is in the Administrators group, are what it records on Windows.
type Identity struct {
	UID   int
	RunAs string
	Admin bool
}

// Summary is everything collection.json and the stdout summary report.
// Started and Finished are UTC times already formatted. Users, Homes and
// ActiveUsers are parallel lists of the enumerated homes and of those that gave
// a row; Projects holds project directories only. Root is "" in a live run.
type Summary struct {
	Version     string
	Hostname    string
	Mode        string
	Root        string
	Platform    string
	Identity    Identity
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
// unreadable-directory row, in order, as list_active_users does. hasRows is
// manifest.Writer.HasHomeRows.
func ActiveUsers(homes []users.Home, hasRows func(home string) bool) []users.Home {
	var active []users.Home
	for _, home := range homes {
		if home.Path != "" && hasRows(home.Path) {
			active = append(active, home)
		}
	}
	return active
}

// WriteCollectionJSON writes collection.json through encoding/json: the
// fields in the documented order, indented by two spaces, every object and list
// with its items on lines of their own.
func WriteCollectionJSON(w io.Writer, summary Summary) error {
	return encodeDocument(w, summary)
}
