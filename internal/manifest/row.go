// Package manifest writes the manifest, one JSON object per line with
// encoding/json in the documented field order, and keeps the counts the
// summary reports.
package manifest

// Status is the outcome recorded for one path.
type Status string

// The nine statuses a row can have.
const (
	Collected              Status = "collected"
	Symlink                Status = "symlink"
	SkippedExcluded        Status = "skipped_excluded"
	SkippedSize            Status = "skipped_size"
	SkippedSecret          Status = "skipped_secret"
	SkippedSpecial         Status = "skipped_special"
	ErrorCopy              Status = "error_copy"
	SkippedUnmatchedVolume Status = "skipped_unmatched_volume"
	ErrorRead              Status = "error_read"
)

// Statuses returns every status in the order collection.json counts them.
func Statuses() []Status {
	return []Status{
		Collected, Symlink, SkippedExcluded, SkippedSize, SkippedSecret,
		SkippedSpecial, ErrorCopy, SkippedUnmatchedVolume, ErrorRead,
	}
}

// Row is one manifest line. The fields are in manifest order; Type is
// "file", "symlink", "dir", or the name of a special file.
type Row struct {
	User        string
	Home        string
	Agent       string
	Path        string
	ArchivePath string
	Type        string
	Size        int64
	Mtime       int64
	Atime       int64
	Ctime       int64
	Btime       int64
	UID         uint32
	GID         uint32
	Mode        string
	// Owner is the owning account as DOMAIN\name, the security ID text when
	// the name cannot be translated, or "". It is written on Windows rows only.
	Owner string
	// Attributes is the .NET FileAttributes text, such as "Archive, ReparsePoint".
	// It is written on Windows rows only.
	Attributes string
	SHA256     string
	Secret     bool
	Status     Status
	Target     string
	Error      string
}

// jsonRow is the JSON form of a Row: the manifest's fields in their documented
// order. Every field is written, an empty one as "" or 0, so no field has
// omitempty; the platform's own fields (owner and attributes on Windows,
// none elsewhere) come after mode. The numbers are never negative.
type jsonRow struct {
	User        string `json:"user"`
	Home        string `json:"home"`
	Agent       string `json:"agent"`
	Path        string `json:"path"`
	ArchivePath string `json:"archive_path"`
	Type        string `json:"type"`
	Size        int64  `json:"size"`
	Mtime       int64  `json:"mtime"`
	Atime       int64  `json:"atime"`
	Ctime       int64  `json:"ctime"`
	Btime       int64  `json:"btime"`
	UID         uint32 `json:"uid"`
	GID         uint32 `json:"gid"`
	Mode        string `json:"mode"`
	platformFields
	SHA256 string `json:"sha256"`
	Secret bool   `json:"secret"`
	Status Status `json:"status"`
	Target string `json:"target"`
	Error  string `json:"error"`
}

// jsonRowOf returns the JSON form of row. A negative number is written as 0,
// as is every number that is not a run of digits; a time before 1970 is
// negative.
func jsonRowOf(row Row) jsonRow {
	return jsonRow{
		User: row.User, Home: row.Home, Agent: row.Agent, Path: row.Path,
		ArchivePath: row.ArchivePath, Type: row.Type, Size: max(row.Size, 0),
		Mtime: max(row.Mtime, 0), Atime: max(row.Atime, 0),
		Ctime: max(row.Ctime, 0), Btime: max(row.Btime, 0),
		UID: row.UID, GID: row.GID, Mode: row.Mode, platformFields: platformOf(row),
		SHA256: row.SHA256, Secret: row.Secret, Status: row.Status,
		Target: row.Target, Error: row.Error,
	}
}
