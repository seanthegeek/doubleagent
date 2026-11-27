package summary

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/manifest"
)

// humanSize returns bytes in binary units with one decimal, "512 B" or
// "20.9 MiB".
func humanSize(bytes int64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	value := float64(max(bytes, 0))
	index := 0
	for value >= 1024 && index < len(units)-1 {
		value /= 1024
		index++
	}
	if index == 0 {
		return strconv.FormatInt(max(bytes, 0), 10) + " B"
	}
	return strconv.FormatFloat(value, 'f', 1, 64) + " " + units[index]
}

// countNoun returns "N noun", the noun plural with an s unless N is 1.
func countNoun(count int64, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	return strconv.FormatInt(count, 10) + " " + noun + "s"
}

// relocationLine is the value of the relocation line: the hint and file
// counts, then the skipped, resolved and see-also parts when they apply.
func relocationLine(relocation Relocation) string {
	hints := int64(len(relocation.Hints))
	text := countNoun(hints, "hint") + " in " +
		countNoun(int64(relocation.FilesSearched), "file") + " searched"
	if relocation.FilesSkippedSize != 0 {
		text += ", " + strconv.Itoa(relocation.FilesSkippedSize) + " skipped for size"
	}
	if relocation.Resolved != 0 {
		text += ", " + strconv.Itoa(relocation.Resolved) + " resolved"
	}
	if hints != 0 {
		text += "; see docs/relocation.md"
	}
	return text
}

// dockerLine is the value of the docker line.
func dockerLine(summary Summary) string {
	docker := summary.Docker
	text := countNoun(int64(docker.Found), "volume") + " found, " +
		strconv.Itoa(docker.Collected) + " collected, " +
		strconv.Itoa(docker.Unreadable) + " unreadable"
	if docker.Unreadable > 0 {
		text += " (run as root to collect)"
	}
	if docker.Desktop {
		text += "; Docker Desktop VM disk not collected"
	}
	return text
}

// PrintResult writes the stdout summary, one "name: value" line each and a
// docker line when Docker or Podman data was seen.
func PrintResult(w io.Writer, summary Summary) error {
	counts := summary.Counts
	status := func(which manifest.Status) int64 { return int64(counts.ByStatus[which]) }
	copyErrors, readErrors := status(manifest.ErrorCopy), status(manifest.ErrorRead)
	var out strings.Builder
	line := func(name string, value string) {
		out.WriteString(name + strings.Repeat(" ", 12-len(name)) + value + "\n")
	}
	line("archive:", summary.ArchivePath)
	line("size:", strconv.FormatInt(summary.ArchiveSize, 10)+" ("+humanSize(summary.ArchiveSize)+")")
	line("sha256:", summary.ArchiveSHA)
	line("manifest:", summary.ManifestPath)
	line("summary:", summary.SummaryPath)
	line("log:", summary.LogPath)
	line("users:", strconv.Itoa(len(list(summary.ActiveUsers))))
	line("projects:", strconv.Itoa(len(list(summary.Projects))))
	line("collected:", countNoun(status(manifest.Collected), "file")+", "+
		countNoun(counts.CollectedBytes, "byte")+" ("+humanSize(counts.CollectedBytes)+")")
	line("skipped:", fmt.Sprintf("%d excluded, %d too large, %d secret, %d special",
		status(manifest.SkippedExcluded), status(manifest.SkippedSize),
		status(manifest.SkippedSecret), status(manifest.SkippedSpecial)))
	line("errors:", fmt.Sprintf("%d (%d error_copy, %d error_read)",
		copyErrors+readErrors, copyErrors, readErrors))
	line("relocation:", relocationLine(summary.Relocation))
	if summary.Docker.Seen {
		line("docker:", dockerLine(summary))
	}
	_, err := io.WriteString(w, out.String())
	return err
}
