# Analysis options

These are every option of the analysis commands, `detect`, `timeline`,
`fleet` and `catalog`, with their defaults and the exit codes that automation
can test. `doubleagent <command> --help` prints the same list.

## Command lines

```text
doubleagent detect [options] INPUT
doubleagent timeline [options] INPUT OUTPUT_DIR
doubleagent fleet [-o CSV] INPUT [INPUT ...]
doubleagent catalog [--agents]
doubleagent --version
doubleagent --help
```

- Options come before the input. An argument that starts with `-` after
  the first input is refused with `options go before the input; <argument>
  is after it`. To give an input whose name starts with `-`, end the
  options with `--` first.
- An option may be written with one dash or two (`-json` or `--json`), and
  an option that takes a value takes it as the next argument or after `=`
  (`--since 3d`, `--since=3d`). An option that takes no value is never
  followed by one.
- `INPUT` of `detect` and `timeline` is a collection archive, an extracted
  collection or any directory ([Inputs and detection](inputs.md)). On Linux
  and macOS an input is opened by its absolute path, so a relative `INPUT`
  fails with `file name too long` when the current directory and `INPUT`
  together are longer than the system allows; run the command from a
  shorter directory.
- `OUTPUT_DIR` of `timeline` is a directory that does not exist yet, or an
  empty one, outside the input. It is created with any missing parents,
  and those are removed again when the run fails before writing into them.
  A symbolic link is refused as not a directory. The path is cleaned by
  name before it is checked, so `link/../out` names `out`.
- Each `INPUT` of `fleet` is a saved output of
  `doubleagent collect --inventory`, or a directory of them
  ([Fleet inventory](fleet.md)).

## Option reference

| Option | Commands | Default | Meaning |
| --- | --- | --- | --- |
| `--json` | `detect` | off | Print one JSON object instead of the tables; its keys are in [detect --json](timeline-output.md#detect---json). |
| `--files` | `detect` | off | Also list every attributed file, with its user and agent. With `--json`, a `files` array. |
| `--include-thinking` | `timeline` | off | Write the agents' thinking and reasoning blocks as rows of `turn_type` `thinking`. |
| `--agent AGENT` | `timeline` | every agent with a parser | Run only the parsers of this agent. Repeatable. A name no parser reads stops the command with `--agent <name>: no parser reads this agent`. `doubleagent catalog --agents` lists the names. |
| `--since WHEN` | `timeline` | none | Keep rows at or after `WHEN`: the first millisecond of the period it names. The forms are in [WHEN forms](filtering.md#when-forms). |
| `--until WHEN` | `timeline` | none | Keep rows at or before `WHEN`: the last millisecond of the period it names. |
| `--keep-undated` | `timeline` | off | With `--since` or `--until`, also keep rows that have no timestamp. |
| `--match REGEX` | `timeline` | none | Keep rows whose text matches this Go regular expression, with the tool call or result that pairs with each. Repeatable; a row that matches any pattern is kept. See [Matching text](filtering.md#matching-text). |
| `-i`, `--ignore-case` | `timeline` | off | Make every `--match` pattern ignore case. |
| `--host HOST` | `detect`, `timeline` | the collection's host name | Host name to record for every home. It replaces the `hostname` of `collection.json` when the input has one; a loose tree has none, and shows `(unknown)` without this option. |
| `--user USER` | `detect`, `timeline` | none | User name to record for each home whose user cannot be told from the manifest or the path. |
| `--work-dir DIR` | `detect`, `timeline` | the system's temporary directory | Where an archive is extracted: a new `doubleagent-*` directory is made inside `DIR`, which is created if missing. It is removed when the command ends unless `--keep-extracted` is given, and so are the directories made for `DIR` while they are empty. An empty `DIR` is refused. |
| `--keep-extracted` | `detect`, `timeline` | off | Keep the extracted archive, and print `extracted to: <path>` on stderr. An interrupted run removes it anyway. |
| `--no-secrets` | `detect`, `timeline` | off | Replace the secrets the command identifies with a placeholder. `timeline` replaces the credentials of URLs, the value of a URL parameter named as a credential (`token`, `key`, `sig` and the like), the tokens, keys, passwords and cookie values the parsers identify by the field that holds them, and the detail of a reader's error that could quote a file holding keys. `detect` reads none of the agents' files, so it replaces only URL credentials and credential parameters, and a token in a file name or path stays. A credential in free text that nothing identifies, such as one pasted into a prompt, is shown either way. [Secrets](timeline-output.md#secrets) gives the full rule. |
| `--redaction-placeholder TEXT` | `detect`, `timeline` | `REDACTED` | With `--no-secrets`, the text each secret is replaced by; `""` leaves nothing in its place. Without `--no-secrets` it is refused. |
| `-o CSV`, `--output CSV` | `fleet` | `fleet-inventory.csv` in the current directory | The CSV to write; missing parent directories are created. Refused when it is a symbolic link, one of the input files, or inside an input directory. |
| `--agents` | `catalog` | off | Print each catalog agent with `parser` or `detect only` instead of the catalog text. |
| `-h`, `--help` | every command | | Print the command's help and exit `0`. |
| `-V`, `--version` | `doubleagent` itself | | Print `doubleagent <version>` and exit `0`. |

Without `--agents`, `catalog` prints the bundled catalog as it is, the same
text `doubleagent collect --list` prints.

## Exit codes

| Exit code | `detect` | `timeline` | `fleet` | `catalog` |
| --- | --- | --- | --- | --- |
| `0` | Agent files were found, or `--json` was given. | At least one row was written. | At least one row was written. | Always, unless an error stops it. |
| `1` | No agent files were found. | No row was written, including when the filters dropped every row; the three files are still written. | No `host` or `agent` line was found; the CSV is written with its header only. | |
| `2` | An error or a command-line mistake. | An error or a command-line mistake. | An error or a command-line mistake. | An error or a command-line mistake. |
| `130` | Interrupted. | Interrupted. | Interrupted. | |

A command-line mistake prints `doubleagent <command>: <message>` and the
command's help on stderr. Among them: an unknown option, a missing or extra
argument, an empty input, an option after the input, an output directory
that is not empty, not a directory, inside the input or cannot be written
to, and `--redaction-placeholder` without `--no-secrets`.

Any other error prints one line on stderr, `error: <message>`. These stop
a command with `2`:

- the input cannot be opened, listed or read, as `error: <path>: <reason>`
  ([Unreadable inputs](inputs.md#unreadable-inputs));
- a file input is neither a tar nor a zip archive, or cannot be extracted
  (`<path>: cannot extract: <reason>`), or is neither a regular file nor a
  directory;
- a `--since` or `--until` value is not one of the accepted forms, or
  `--since` is later than `--until`;
- a `--match` pattern is not a valid Go regular expression;
- an `--agent` name that no parser reads;
- a `fleet` input that is a symbolic link, neither a file nor a directory,
  or cannot be read, and a CSV that cannot be written;
- an output file that cannot be written.

The time, pattern and agent checks run before the input is opened, so a
mistyped value costs no extraction.

A run interrupted with Ctrl-C (`SIGINT`) or `SIGTERM` prints `interrupted`
and exits `130`. It removes the extracted archive whatever
`--keep-extracted` says; `fleet` stops before its next input file and writes
no CSV.

`doubleagent` with no command, or with a name that is not a command,
prints the list of commands on stderr and exits `1`.
