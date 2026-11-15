// Package manifest writes the manifest, one JSON object per line, by hand in
// the field order and byte layout of the sh collector's write_manifest_row,
// and keeps the counts the summary reports.
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
	SHA256      string
	Secret      bool
	Status      Status
	Target      string
	Error       string
}
