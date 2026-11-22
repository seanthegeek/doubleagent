# Security policy

DoubleAgent is a forensic tool. `doubleagent collect` gathers the
transcripts, configuration and credentials that AI agents leave on a host,
and the analysis commands read collections that may come from a
compromised system. This page says which versions receive fixes, what
counts as a vulnerability in DoubleAgent itself, and how to report one.

## Supported versions

Only the newest release of `doubleagent`, the newest `vX.Y.Z` tag, is
supported. A report against an older version is first reproduced against
the newest; a fix is released as a new version, never backported. Until
the first release is published, reports against the default branch,
`main`, are welcome.

| Version | Supported |
| --- | --- |
| The newest `vX.Y.Z` release | Yes |
| Any earlier release | No |

`doubleagent --version` prints the running version as
`doubleagent <version>`; a build made from a checkout without a version
prints `0.0.0-dev` and the commit.

## What is a vulnerability

A security issue is a way DoubleAgent itself can cause harm or be made to
misbehave. Reports in these classes are wanted:

- **Vulnerable dependencies.** A known vulnerability in a Go module the
  `doubleagent` program is built with (`go.mod`, and the copies under
  `vendor/`), in a package the documentation site is built with
  (`requirements-docs.txt`), or in an action the GitHub workflows use.
- **Unintended actions.** Anything DoubleAgent does that its documentation
  says it never does: modifying, moving or deleting a file outside the
  output directory; following a symbolic link; writing outside the output
  or extraction directory when it reads an archive, such as an archive
  entry that escapes it (path traversal); printing a file's contents or a
  credential to the terminal or the log of a collection run; or executing
  anything found in the data it reads.
- **Harm to the host or the analyst workstation.** A crafted file tree,
  archive, transcript or database that makes `doubleagent` hang, exhaust
  memory or disk, or crash in a way that leaves its output in a misleading
  state (a denial of service), including through compressed (gzip, zstd),
  protobuf or SQLite data.
- **A secret shown that `--no-secrets` promises to withhold.** A value the
  [output page](docs/timeline-output.md) says `--no-secrets` replaces that
  appears in the output of a run given `--no-secrets`.

These are not vulnerabilities in DoubleAgent:

- **That it works.** `doubleagent collect` copies credentials, tokens and
  transcripts from every user on a host, and the analysis commands read
  them. That is the purpose, documented on these pages, and the same is
  true of most forensic tools. Misuse by someone who already has the
  access the tool needs (root or Administrator on the host, or a copy of a
  collection) is a matter for that person's authorisation, not a flaw in
  the tool.
- **What the agents do.** A credential an agent stores in clear text, a
  transcript that records a secret, or an agent that writes somewhere
  surprising is a finding about that agent. It belongs in the agent's
  research document and an ordinary issue, not a security report.
- **Gaps in coverage.** An agent the catalog misses, a path a collection
  run does not gather or a record `doubleagent timeline` does not parse is
  a feature request.
- **The cost of `--match`.** A `--match` pattern is searched in a row's
  full text, including a long text that many rows share, so many rows that
  name one long text cost that text once per row. This is accepted so that
  a pattern can match across the boundary between a row's own words and
  the shared text. The pattern is the analyst's own, and Go's regular
  expressions run in time linear in the text they search.
  [Filtering the timeline](docs/filtering.md) describes the cost.
- **Secrets shown by default.** The analysis commands show the credentials
  they find in the evidence as the evidence holds them, unless
  `--no-secrets` is given, because a credential helps establish what an
  agent could reach. `--no-secrets` withholds only what the
  [output page](docs/timeline-output.md) says it withholds; a credential
  in free text that no rule identifies, such as a token pasted into a
  prompt, is shown either way.

If you are unsure which it is, report it privately; a report that turns out
to be an ordinary bug is moved to an issue with your agreement.

## Reporting a vulnerability

Report privately through GitHub's advisory form,
[Report a vulnerability](https://github.com/seanthegeek/doubleagent/security/advisories/new),
and not in a public issue, pull request or discussion. Please include:

- the command line as run, options included;
- the output of `doubleagent --version`;
- the platform: operating system name and version, and architecture
  (x86-64 or arm64);
- what happened and what the documentation says should have happened;
- steps to reproduce, with a synthetic input. Never attach a real
  collection: it holds credentials and transcripts from the host it came
  from. Build the reproduction the way the tests build theirs: the fake
  disk image the collection tests run over (`internal/collect/imagetest`)
  and the parsers' synthetic files under each package's `testdata`.

You will get an acknowledgement within seven days. The fix is developed in
private, released as a new version with a changelog entry that names the
problem, and published as a GitHub security advisory, with a CVE when one
applies and credit to the reporter unless you ask otherwise. Please keep the
report private until the advisory is published; a release date is agreed
with you before then. There is no bug bounty.

Dependency alerts from Dependabot are reviewed before every release; a
release is not made while an alert is open.
