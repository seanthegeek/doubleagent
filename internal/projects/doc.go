// Package projects finds the project directories named in the agents' state
// under each home, gives each its owner and keeps those that are
// directories. The readers match lines, as grep and sed do, rather than
// parse JSON, YAML or TOML, so a damaged or unusual file still gives the
// paths it names, in a fixed order.
package projects
