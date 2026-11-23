// Package catalog holds the seven tables that say what `collect` gathers:
// the home and project artifacts, the size exclusions, the credential files,
// the Docker volumes, the relocation variables and the startup files. The
// tables are one embedded text file in the format `collect --list` prints, so
// no path or glob lives in code.
package catalog
