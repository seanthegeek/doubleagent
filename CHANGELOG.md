# Changelog

This file follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and DoubleAgent's versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- The first release of `doubleagent`, one program with the commands `collect`, `detect`, `timeline`, `fleet` and `catalog`. If you used the earlier collector scripts and Python analyzer, [Differences from the scripts](docs/differences-from-the-scripts.md) lists what changed.

### Changed

- The analysis commands read an agent's TOML files as TOML 1.1, which adds to TOML 1.0 a newline or a trailing comma inside an inline table, a time or date-time without seconds, and the `\x` and `\e` string escapes: a file using one of these now gives its rows where it gave one error row. Every TOML 1.0 file reads as before ([#569]).
- `collect` keeps its staging directory open for the whole run and stages, archives and removes through it, so moving the directory aside and putting a link at its name during the run no longer sends staged copies through the link; on Windows the directory cannot be renamed or deleted while the run works, and on a volume that keeps access lists (NTFS) it is made with its own, the running account and Administrators only, while on FAT, exFAT or a network share it is made as any other directory there. A run now ends with exit `2` (`Cannot create staging dir: ...`) when the staging directory it has just made and opened is not an empty directory it can take as its own, and leaves that directory in the output directory ([#542]).
- When several Docker volume patterns match one volume name, the most specific one now decides which agent the volume is collected as: the pattern with the fewest `*`, then the one with the most literal characters, then the one listed first, where the first pattern listed won before. A new `*ollama_data` pattern keeps a Local Deep Research compose project's `local-deep-research_ollama_data` volume with `ollama`. A volume name that holds the names of two agents can now go to the other agent, for example `ollama_n8n_data` to `n8n` rather than `ollama` ([#583]).

### Fixed

- Whether a TOML file was read as TOML 1.0 or TOML 1.1 depended on the `BURNTSUSHI_TOML_110` environment variable of the analyst's shell, so one collection could give two timelines; the variable no longer has any effect ([#569]).
- A TOML file that defines one key twice as an array is refused with an error row; the last array was kept silently ([#569]).
- A TOML file with an array of inline tables, one of them holding the empty key `""`, could give its settings rows one merged table with a `<nil>` value, or drop tables of the array; every row now shows the values the file holds, and such an array's tables list their keys sorted ([#570]).
- A TOML file the TOML 1.1 rules make invalid gave settings rows instead of an error row when it defined a key again after dotted keys made it a table, added keys to an array, to an inline table or to a table defined in another part of the file, or wrote a date-time with an offset hour over 23 or minute over 59; the rows could miss a value the file holds or show an offset of `-00:60` as `-01:00`. Such a file now gives an error row naming the line and column ([#571]).
- An excluded path that is itself a symbolic link (on Windows also a junction) now has a `skipped_excluded` row typed `symlink`, with the link's own metadata and size and its stored `target`, where it had a `dir` row when it led to a directory and a `file` row otherwise, both with an empty `target`; the link is never followed. The manifest interface changed: a `skipped_excluded` row can be typed `symlink` and carry a `target`. Based on [pull request 508](https://github.com/seanthegeek/doubleagent/pull/508) ([#507]).
- On Linux and macOS, when catalog lines that differ only in letter case (`.config/goose` and `.config/Goose`) matched one directory on a case-insensitive file system, `collect` spelled the manifest `path` and the archive member as the line listed first spells it, so the spelling, and which `EXCLUDES` and `SECRET_GLOBS` lines applied, depended on the order of the catalog lines. The path now carries the spelling the directory holds, as on Windows, and the `EXCLUDES` and `SECRET_GLOBS` lines spelled as the disk is apply; the match still belongs to the line listed first. The log line for the dropped line now reads `[agent] <path>: catalog line <glob> names the same file as <kept path> (case-insensitive filesystem), collected there` ([#584]).

[Unreleased]: https://github.com/seanthegeek/doubleagent/commits/main
[#584]: https://github.com/seanthegeek/doubleagent/issues/584
[#583]: https://github.com/seanthegeek/doubleagent/issues/583
[#571]: https://github.com/seanthegeek/doubleagent/issues/571
[#570]: https://github.com/seanthegeek/doubleagent/issues/570
[#569]: https://github.com/seanthegeek/doubleagent/issues/569
[#542]: https://github.com/seanthegeek/doubleagent/issues/542
[#507]: https://github.com/seanthegeek/doubleagent/issues/507
