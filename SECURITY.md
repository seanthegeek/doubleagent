# Security policy

DoubleAgent is a forensic tool. Its collectors gather the transcripts,
configuration and credentials that AI agents leave on a host, and its
analyzer parses collections that may come from a compromised system. This
page says which versions receive fixes, what counts as a vulnerability in
the tool itself, and how to report one.

## Supported versions

Only the latest release of each part is supported. The collectors and the
analyzer are versioned and tagged separately: the newest `collector-vX.Y.Z`
tag covers `collect-agent-artifacts.sh` and `Collect-AgentArtifacts.ps1`,
and the newest `analyzer-vX.Y.Z` tag covers the `doubleagent` Python
package. A report against an older version is first reproduced against the
latest; a fix is released as a new tag, never backported.

| Part | Supported |
| --- | --- |
| Collectors, the newest `collector-vX.Y.Z` tag | Yes |
| Analyzer, the newest `analyzer-vX.Y.Z` tag | Yes |
| Any earlier tag | No |

The running version is printed by `collect-agent-artifacts.sh --version`,
`Collect-AgentArtifacts.ps1 -Version` and `doubleagent --version`.

## What is a vulnerability

A security issue is a way the tool itself can cause harm or be made to
misbehave. Reports in these classes are wanted:

- **Vulnerable dependencies.** A known vulnerability in a library the
  `doubleagent` binary or the documentation site depends on (`go.mod`,
  `requirements-docs.txt`) or in an action the workflows use. The collectors
  have no dependencies beyond the base system.
- **Unintended actions.** Anything the collectors or the analyzer do that the
  documentation says they never do: modifying, moving or deleting a file
  outside the output directory, following a symlink, materialising a symlink
  or writing outside the output directory when extracting an archive,
  printing a credential or file contents to stdout or a log, executing
  anything found in the data they read, or an archive entry that escapes the
  output directory (path traversal).
- **Harm to the host or the analyst workstation.** A crafted file tree,
  archive, transcript or database that makes a collector or the analyzer hang,
  exhaust memory or disk, or crash in a way that leaves the output in a
  misleading state (a denial of service), including through the handling of
  compressed or protobuf data.

These are not vulnerabilities in DoubleAgent:

- **That it works.** The collectors copy credentials, tokens and transcripts
  from every user on a host, and the analyzer reads them. That is the
  purpose, documented in the READMEs, and the same is true of most forensic
  tooling. Misuse by someone who already has the access the tool needs (root
  or Administrator on the host, or a copy of a collection) is a matter for
  that person's authorisation, not a flaw in the tool.
- **What the agents do.** A credential an agent stores in clear text, a
  transcript that records a secret, or an agent that writes somewhere
  surprising is a finding about that agent. It belongs in the agent's
  research document and an ordinary issue, not a security report.
- **Gaps in coverage.** An agent the catalog misses, a path the collector
  does not gather or a record the analyzer does not parse is a feature
  request.

If you are unsure which it is, report it privately; a report that turns out
to be an ordinary bug is moved to an issue with your agreement.

## Reporting a vulnerability

Report privately through GitHub's advisory form,
[Report a vulnerability](https://github.com/seanthegeek/doubleagent/security/advisories/new),
and not in a public issue, pull request or discussion. Please include:

- the part (collector or analyzer) and the version;
- the platform: operating system name and version, architecture (x86_64 or
  arm64), the shell (`sh`, `dash`, `bash`, busybox `ash`) or PowerShell
  version for the collectors, the Python version for the analyzer;
- what happened and what the documentation says should have happened;
- steps to reproduce, with a synthetic input. Never attach a real collection:
  it contains credentials and transcripts from the host it came from. The
  smoke tests under `collectors/tests/` build a fake disk image and the Go
  tests build synthetic homes under a temporary directory; a reproduction
  built the same way is the most useful form.

You will get an acknowledgement within seven days. The fix is developed in
private, released as a new tag with a changelog entry that names the
problem, and published as a GitHub security advisory, with a CVE when one
applies and credit to the reporter unless you ask otherwise. Please keep the
report private until the advisory is published; a release date is agreed
with you before then. There is no bug bounty.

Dependency alerts from Dependabot are reviewed before every release; a
release is not tagged while an alert is open.
