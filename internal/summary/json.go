package summary

import (
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/manifest"
)

// quoted returns text as a JSON string.
func quoted(text string) string { return `"` + manifest.EscapeJSON(text) + `"` }

// jsonBool returns true or false as JSON writes them.
func jsonBool(flag bool) string { return strconv.FormatBool(flag) }

// jsonList returns the non-empty items as a JSON array with no spaces, as
// json_list does.
func jsonList(items []string) string {
	out := "["
	started := false
	for _, item := range items {
		if item == "" {
			continue
		}
		if started {
			out += ","
		}
		out += quoted(item)
		started = true
	}
	return out + "]"
}

// field returns `"key": value` as the template spaces it.
func field(key, value string) string { return `"` + key + `": ` + value }

// collectionJSON builds the document of write_summary's EOF_SUMMARY template.
func collectionJSON(summary Summary) []byte {
	root := summary.Root
	if root == "" {
		root = "/"
	}
	text := "{\n" +
		"  " + field("tool", quoted(Tool)) + ",\n" +
		"  " + field("version", quoted(summary.Version)) + ",\n" +
		"  " + field("hostname", quoted(summary.Hostname)) + ",\n" +
		"  " + field("mode", quoted(summary.Mode)) + ",\n" +
		"  " + field("root", quoted(root)) + ",\n" +
		"  " + field("platform", quoted(summary.Platform)) + ",\n" +
		"  " + field("run_as_uid", quoted(strconv.Itoa(summary.UID))) + ",\n" +
		"  " + field("started", quoted(summary.Started)) + ",\n" +
		"  " + field("finished", quoted(summary.Finished)) + ",\n" +
		"  " + field("options", optionsJSON(summary.Options)) + ",\n" +
		"  " + field("capabilities", capabilitiesJSON(summary.Plutil)) + ",\n" +
		"  " + field("users", jsonList(summary.Users)) + ",\n" +
		"  " + field("homes", jsonList(summary.Homes)) + ",\n" +
		"  " + field("users_with_artifacts", jsonList(summary.ActiveUsers)) + ",\n" +
		"  " + field("projects", jsonList(summary.Projects)) + ",\n" +
		"  " + field("counts", countsJSON(summary.Counts)) + ",\n" +
		"  " + field("docker", dockerJSON(summary)) + ",\n" +
		"  " + field("relocation", relocationJSON(summary.Relocation)) + ",\n" +
		"  " + field("notes", jsonList(summary.Notes)) + ",\n" +
		"  " + field("archive", quoted(summary.Archive)) + "\n}\n"
	return []byte(text)
}

// joinFields joins `"key": value` pairs as the template's single-line objects
// do, between braces.
func joinFields(fields ...string) string {
	out := "{"
	for index, text := range fields {
		if index > 0 {
			out += ", "
		}
		out += text
	}
	return out + "}"
}

// optionsJSON builds the options object.
func optionsJSON(options Options) string {
	return joinFields(
		field("full", jsonBool(options.Full)),
		field("no_secrets", jsonBool(options.NoSecrets)),
		field("max_file_size_bytes", strconv.FormatInt(options.MaxFileBytes, 10)),
		field("users_filter", quoted(options.UsersFilter)),
		field("no_docker", jsonBool(options.NoDocker)),
		field("no_projects", jsonBool(options.NoProjects)),
		field("projects", jsonList(options.Projects)),
		field("env_scan_home", jsonBool(options.EnvScanHome)),
		field("env_scan_max_size_bytes", strconv.FormatInt(options.EnvScanMaxSize, 10)),
	)
}

// capabilitiesJSON builds the capabilities object. The hash and archive code
// is built in, so those values never vary; stat_mode and worker_shell have no
// Go counterpart and are not written.
func capabilitiesJSON(plutil bool) string {
	return joinFields(
		field("hash_tool", quoted("crypto/sha256")),
		field("archiver", quoted("archive/tar")),
		field("grep_text", jsonBool(true)),
		field("plutil", jsonBool(plutil)),
	)
}

// countsJSON builds the counts object: one count per status, in order, then
// the collected bytes.
func countsJSON(counts manifest.Counts) string {
	var fields []string
	for _, status := range manifest.Statuses() {
		fields = append(fields, field(string(status), strconv.Itoa(counts.ByStatus[status])))
	}
	fields = append(fields, field("collected_bytes", strconv.FormatInt(counts.CollectedBytes, 10)))
	return joinFields(fields...)
}

// dockerJSON builds the docker object.
func dockerJSON(summary Summary) string {
	return joinFields(
		field("volumes_found", strconv.Itoa(summary.Docker.Found)),
		field("volumes_collected", strconv.Itoa(summary.Docker.Collected)),
		field("unreadable", strconv.Itoa(summary.Docker.Unreadable)),
		field("docker_desktop", jsonBool(summary.Docker.Desktop)),
	)
}

// relocationJSON builds the relocation object, one hint per line.
func relocationJSON(relocation Relocation) string {
	out := `{"hints": [`
	for index, hint := range relocation.Hints {
		if index > 0 {
			out += ","
		}
		out += "\n    " + joinFields(
			field("user", quoted(hint.User)),
			field("source", quoted(hint.Source)),
			field("line", strconv.Itoa(hint.Line)),
			field("variable", quoted(hint.Variable)),
			field("agent", quoted(hint.Agent)),
			field("value", quoted(hint.Value)),
		)
	}
	if len(relocation.Hints) > 0 {
		out += "\n  "
	}
	out += "], " + field("registry", joinFields(
		field("read", jsonBool(relocation.RegistryRead)),
		field("reason", quoted(relocation.RegistryReason)),
		field("users_read", "[]"),
		field("users_not_loaded", "[]"),
		field("users_denied", "[]"),
	)) + ", "
	out += field("files_searched", strconv.Itoa(relocation.FilesSearched)) + ", "
	return out + field("files_skipped_size", strconv.Itoa(relocation.FilesSkippedSize)) + "}"
}
