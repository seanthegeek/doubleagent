package summary

import (
	"encoding/json"
	"io"

	"github.com/seanthegeek/doubleagent/internal/manifest"
)

// document is the JSON form of collection.json: the fields in their
// documented order. Every field is written, an empty list as [] and an empty
// text as "", so no field has omitempty. The account fields differ by
// platform (identity_unix.go, identity_windows.go).
type document struct {
	Tool     string `json:"tool"`
	Version  string `json:"version"`
	Hostname string `json:"hostname"`
	Mode     string `json:"mode"`
	Root     string `json:"root"`
	Platform string `json:"platform"`
	account
	Started      string       `json:"started"`
	Finished     string       `json:"finished"`
	Options      optionsJSON  `json:"options"`
	Capabilities capabilities `json:"capabilities"`
	Users        []string     `json:"users"`
	Homes        []string     `json:"homes"`
	Active       []string     `json:"users_with_artifacts"`
	Projects     []string     `json:"projects"`
	Counts       countsJSON   `json:"counts"`
	Docker       dockerJSON   `json:"docker"`
	Relocation   relocation   `json:"relocation"`
	Notes        []string     `json:"notes"`
	Archive      string       `json:"archive"`
}

// optionsJSON is the options object.
type optionsJSON struct {
	Full            bool     `json:"full"`
	NoSecrets       bool     `json:"no_secrets"`
	MaxFileBytes    int64    `json:"max_file_size_bytes"`
	UsersFilter     string   `json:"users_filter"`
	NoDocker        bool     `json:"no_docker"`
	NoProjects      bool     `json:"no_projects"`
	Projects        []string `json:"projects"`
	EnvScanHome     bool     `json:"env_scan_home"`
	EnvScanMaxSize  int64    `json:"env_scan_max_size_bytes"`
	NoPreserveAtime bool     `json:"no_preserve_atime"`
}

// capabilities is the capabilities object. The hash and archive code is
// built in, so those values never vary; stat_mode and worker_shell have no Go
// counterpart and are not written.
type capabilities struct {
	HashTool string `json:"hash_tool"`
	Archiver string `json:"archiver"`
	GrepText bool   `json:"grep_text"`
	Plutil   bool   `json:"plutil"`
}

// countsJSON is the counts object: one count per status, in the order of
// manifest.Statuses, then the collected bytes.
type countsJSON struct {
	Collected              int   `json:"collected"`
	Symlink                int   `json:"symlink"`
	SkippedExcluded        int   `json:"skipped_excluded"`
	SkippedSize            int   `json:"skipped_size"`
	SkippedSecret          int   `json:"skipped_secret"`
	SkippedSpecial         int   `json:"skipped_special"`
	ErrorCopy              int   `json:"error_copy"`
	SkippedUnmatchedVolume int   `json:"skipped_unmatched_volume"`
	ErrorRead              int   `json:"error_read"`
	CollectedBytes         int64 `json:"collected_bytes"`
}

// dockerJSON is the docker object.
type dockerJSON struct {
	VolumesFound     int  `json:"volumes_found"`
	VolumesCollected int  `json:"volumes_collected"`
	Unreadable       int  `json:"unreadable"`
	DockerDesktop    bool `json:"docker_desktop"`
}

// relocation is the relocation object.
type relocation struct {
	Hints            []hintJSON   `json:"hints"`
	Registry         registryJSON `json:"registry"`
	FilesSearched    int          `json:"files_searched"`
	FilesSkippedSize int          `json:"files_skipped_size"`
}

// hintJSON is one entry of the relocation hints.
type hintJSON struct {
	User     string `json:"user"`
	Source   string `json:"source"`
	Line     int    `json:"line"`
	Variable string `json:"variable"`
	Agent    string `json:"agent"`
	Value    string `json:"value"`
}

// registryJSON is the relocation registry object.
type registryJSON struct {
	Read           bool     `json:"read"`
	Reason         string   `json:"reason"`
	UsersRead      []string `json:"users_read"`
	UsersNotLoaded []string `json:"users_not_loaded"`
	UsersDenied    []string `json:"users_denied"`
}

// list returns the items that are not empty, as a non-nil slice so an empty
// list is written as [] and not null: the sh collector's json_list leaves out
// an empty item.
func list(items []string) []string {
	kept := []string{}
	for _, item := range items {
		if item != "" {
			kept = append(kept, item)
		}
	}
	return kept
}

// documentOf returns the JSON form of summary.
func documentOf(summary Summary) document {
	root := summary.Root
	if root == "" {
		root = LiveRootText
	}
	return document{
		Tool: Tool, Version: summary.Version, Hostname: summary.Hostname, Mode: summary.Mode,
		Root: root, Platform: summary.Platform, account: accountOf(summary.Identity),
		Started: summary.Started, Finished: summary.Finished,
		Options:      optionsOf(summary.Options),
		Capabilities: capabilities{HashTool: "crypto/sha256", Archiver: "archive/tar", GrepText: true, Plutil: summary.Plutil},
		Users:        list(summary.Users), Homes: list(summary.Homes),
		Active: list(summary.ActiveUsers), Projects: list(summary.Projects),
		Counts: countsOf(summary.Counts), Docker: dockerOf(summary),
		Relocation: relocationOf(summary.Relocation),
		Notes:      list(summary.Notes), Archive: summary.Archive,
	}
}

// optionsOf returns the JSON form of the options.
func optionsOf(options Options) optionsJSON {
	return optionsJSON{
		Full: options.Full, NoSecrets: options.NoSecrets, MaxFileBytes: options.MaxFileBytes,
		UsersFilter: options.UsersFilter, NoDocker: options.NoDocker,
		NoProjects: options.NoProjects, Projects: list(options.Projects),
		EnvScanHome: options.EnvScanHome, EnvScanMaxSize: options.EnvScanMaxSize,
		NoPreserveAtime: options.NoPreserveAtime,
	}
}

// countsOf returns the JSON form of the counts.
func countsOf(counts manifest.Counts) countsJSON {
	status := func(which manifest.Status) int { return counts.ByStatus[which] }
	return countsJSON{
		Collected: status(manifest.Collected), Symlink: status(manifest.Symlink),
		SkippedExcluded: status(manifest.SkippedExcluded), SkippedSize: status(manifest.SkippedSize),
		SkippedSecret: status(manifest.SkippedSecret), SkippedSpecial: status(manifest.SkippedSpecial),
		ErrorCopy:              status(manifest.ErrorCopy),
		SkippedUnmatchedVolume: status(manifest.SkippedUnmatchedVolume),
		ErrorRead:              status(manifest.ErrorRead), CollectedBytes: counts.CollectedBytes,
	}
}

// dockerOf returns the JSON form of the docker counts.
func dockerOf(summary Summary) dockerJSON {
	return dockerJSON{
		VolumesFound: summary.Docker.Found, VolumesCollected: summary.Docker.Collected,
		Unreadable: summary.Docker.Unreadable, DockerDesktop: summary.Docker.Desktop,
	}
}

// relocationOf returns the JSON form of the relocation result.
func relocationOf(result Relocation) relocation {
	hints := make([]hintJSON, 0, len(result.Hints))
	for _, hint := range result.Hints {
		hints = append(hints, hintJSON(hint))
	}
	return relocation{
		Hints: hints,
		Registry: registryJSON{
			Read: result.RegistryRead, Reason: result.RegistryReason,
			UsersRead: list(result.UsersRead), UsersNotLoaded: list(result.UsersNotLoaded),
			UsersDenied: list(result.UsersDenied),
		},
		FilesSearched: result.FilesSearched, FilesSkippedSize: result.FilesSkippedSize,
	}
}

// encodeDocument writes summary to w as one JSON value, indented by two
// spaces, ending with a newline, in one write.
func encodeDocument(w io.Writer, summary Summary) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(documentOf(summary))
}
