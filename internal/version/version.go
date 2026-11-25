package version

import "runtime/debug"

// development is the version of a build no release workflow stamped.
const development = "0.0.0-dev"

// Version is the project version. The release workflow sets it from the tag
// with -ldflags -X; a development build keeps 0.0.0-dev. It is the one
// package-level variable the Go standards allow, because -X needs one.
var Version = development

// String returns Version, with the commit appended to a development build so a
// binary built from a checkout still names the source it came from.
func String() string {
	if Version != development {
		return Version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Version
	}
	return Version + commitSuffix(info.Settings)
}

// commitSuffix returns "+<12 hex digits>" from the build's vcs.revision,
// with "-modified" when the tree was dirty, or "" when no revision is known.
func commitSuffix(settings []debug.BuildSetting) string {
	revision, modified := "", false
	for _, setting := range settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if revision == "" {
		return ""
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if modified {
		revision += "-modified"
	}
	return "+" + revision
}
