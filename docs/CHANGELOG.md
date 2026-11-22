# Changelog

This file follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and DoubleAgent's versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- The first release of `doubleagent`, one program with the commands `collect`, `detect`, `timeline`, `fleet` and `catalog`. If you used the earlier collector scripts and Python analyzer, [Differences from the scripts](differences-from-the-scripts.md) lists what changed.

### Changed

- The analysis commands read an agent's TOML files as TOML 1.1, which adds to TOML 1.0 a newline or a trailing comma inside an inline table, a time or date-time without seconds, and the `\x` and `\e` string escapes: a file using one of these now gives its rows where it gave one error row. Every TOML 1.0 file reads as before ([#569]).

### Fixed

- Whether a TOML file was read as TOML 1.0 or TOML 1.1 depended on the `BURNTSUSHI_TOML_110` environment variable of the analyst's shell, so one collection could give two timelines; the variable no longer has any effect ([#569]).
- A TOML file that defines one key twice as an array is refused with an error row; the last array was kept silently ([#569]).

[Unreleased]: https://github.com/seanthegeek/doubleagent/commits/main
[#569]: https://github.com/seanthegeek/doubleagent/issues/569
