package analyze

import (
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// placed returns path made absolute as the command's own file calls take
// it: input.Absolute, the rule the input is read by, so the output is
// placed where the commands write it. Every Windows file call takes the
// \\?\ form, which has no length limit.
func placed(path string) (string, error) { return input.Absolute(path) }

// followLink returns the directory the link or junction at path leads to,
// every link and junction on the way followed, as collect resolves its
// output directory (stage.PhysicalDir), in native form. A link to anything
// but a directory is an error.
func followLink(path string) (string, error) {
	physical, err := stage.PhysicalDir(path)
	if err != nil {
		return "", err
	}
	return syspath.ToNative(physical), nil
}

// outputParent returns the directory holding path's last element by its
// text, as Windows resolves it, or "" for a path of one element.
func outputParent(path string) string {
	if parent := filepath.Dir(path); parent != "." {
		return parent
	}
	return ""
}

// belowGiven returns the path of name in directory. Windows resolves a ".."
// by the text before it, so filepath.Join, which also keeps a drive-relative
// C: as it is, names the directory that was listed.
func belowGiven(directory, name string) string { return filepath.Join(directory, name) }

// outputNameRefusal returns why an output path cannot be used, or "". On
// Windows a colon after the drive names an alternate data stream of a
// file (an archive member holding one is refused the same way), so an
// output with one could be written as a stream onto an input file or
// directory, which the outside-the-input check cannot see.
func outputNameRefusal(output string) string {
	if strings.Contains(output[len(filepath.VolumeName(output)):], ":") {
		return "the output " + output + " cannot hold a colon after its drive"
	}
	return ""
}
