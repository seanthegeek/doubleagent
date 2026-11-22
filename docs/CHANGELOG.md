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
- A TOML file with an array of inline tables, one of them holding the empty key `""`, could give its settings rows one merged table with a `<nil>` value, or drop tables of the array; every row now shows the values the file holds, and such an array's tables list their keys sorted ([#570]).
- A TOML file the TOML 1.1 rules make invalid gave settings rows instead of an error row when it defined a key again after dotted keys made it a table, added keys to an array, to an inline table or to a table defined in another part of the file, or wrote a date-time with an offset hour over 23 or minute over 59; the rows could miss a value the file holds or show an offset of `-00:60` as `-01:00`. Such a file now gives an error row naming the line and column ([#571]).

[Unreleased]: https://github.com/seanthegeek/doubleagent/commits/main
[#571]: https://github.com/seanthegeek/doubleagent/issues/571
[#570]: https://github.com/seanthegeek/doubleagent/issues/570
[#569]: https://github.com/seanthegeek/doubleagent/issues/569
