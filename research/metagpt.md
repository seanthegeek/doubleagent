# metagpt

MetaGPT is a Python multi-agent "software company" framework with the `metagpt` CLI. It came
into the catalog from the agent framework sweep for
[issue #64](https://github.com/seanthegeek/doubleagent/issues/64); see
[`research/frameworks.md`](frameworks.md) for the frameworks that got no catalog line. It is
collected only, with no parser: MetaGPT keeps no transcripts under the home.

## Source and evidence

[FoundationAgents/MetaGPT](https://github.com/FoundationAgents/MetaGPT)
([geekan/MetaGPT](https://github.com/geekan/MetaGPT) redirects there), MIT
([setup.py:107](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/setup.py#L107)),
open source, commit
[`11cdf466d042aece04fc6cfd13b28e1a70341b1f`](https://github.com/FoundationAgents/MetaGPT/commit/11cdf466d042aece04fc6cfd13b28e1a70341b1f)
(2026-01-21). All claims are from source.

MetaGPT runs shell commands on the host: the `Terminal` tool spawns a
persistent shell
([terminal.py:47-52](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/tools/libs/terminal.py#L47-L52)).

### Observed

- No real install: nothing was installed or run.

### Source reviews

- 2026-10-19: HEAD of FoundationAgents/MetaGPT is still
  [`11cdf46`](https://github.com/FoundationAgents/MetaGPT/commit/11cdf466d042aece04fc6cfd13b28e1a70341b1f),
  re-read from source for permission, model, past-use and sync records. It added the run log and
  the model key ([Per-user storage](#per-user-storage)); there is still nothing to parse.

## Collection

### Per-user storage

| Path (home-relative, all OSes) | Holds | Default or opt-in |
| --- | --- | --- |
| `.metagpt/config2.yaml` (`Path.home()`) | LLM configuration including `api_key` ([const.py:39](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/const.py#L39), [config2.py:115-118](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/config2.py#L115-L118), [software_company.py:130-148](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/software_company.py#L130-L148), [README.md:70-73](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/README.md#L70-L73)) | read by every run; written by `metagpt --init-config` |
| `.metagpt/config2.bak` | the previous config, renamed on re-init ([software_company.py:146-148](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/software_company.py#L146-L148)) | on a second `--init-config` |

`CONFIG_ROOT / "config2.yaml"` is the only home path in the package (grep
for `CONFIG_ROOT`, `Path.home`, `expanduser`, `platformdirs`). The model is `llm.model` in
that file ([llm_config.py:64](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/configs/llm_config.py#L64)).

Nothing records a permission decision: the `Terminal` tool asks nothing before a command (no
`approve` or `confirm` in [terminal.py](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/tools/libs/terminal.py)), and the one
question `RoleZero` asks, whether to continue past the action limit, goes to the human through
the environment and is not stored
([role_zero.py:328-335](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/roles/di/role_zero.py#L328-L335)). Nothing syncs or relays a
session (not found in `metagpt/` at
[`11cdf46`](https://github.com/FoundationAgents/MetaGPT/commit/11cdf466d042aece04fc6cfd13b28e1a70341b1f),
grepped for `share_url`, `upload_session` and `sync_session`), and there is no session store
to delete from.

#### Scratch directories

`<METAGPT_ROOT>/workspace` ([Workspace](#workspace)) is the agent's own output directory for
generated projects; nothing under the home records it, so no catalog line or discovery source can
find it.

#### Relocation variables

| Variable | Moves | Replaces | Table agent |
| --- | --- | --- | --- |
| `METAGPT_PROJECT_ROOT` | the `METAGPT_ROOT` that holds `workspace` and `logs` ([const.py:19-41](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/const.py#L19-L41)) | | `metagpt` |

Not relocation variables: `METAGPT_REPORTER_URL` (carries a URL).

#### Workspace

Generated projects and serialized team state go to
`<METAGPT_ROOT>/workspace` and `workspace/storage/team/team.json`, where
`METAGPT_ROOT` is `$METAGPT_PROJECT_ROOT`, else the package parent if it
holds `.git` (an editable install from a clone), else the working
directory
([const.py:19-41](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/const.py#L19-L41),
[const.py:58](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/const.py#L58),
[workspace_config.py:18-37](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/configs/workspace_config.py#L18-L37),
[team.py:59-65](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/team.py#L59-L65)).
`team.json` is written when `Team.run` ends in an exception or Ctrl-C
([common.py:675-684](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/utils/common.py#L675-L684)).
Every run also writes a DEBUG-level log, `logs/<YYYYMMDD>.txt` (or `<name>_<YYYYMMDD>.txt`),
under the same `METAGPT_ROOT`
([logs.py:40-51](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/logs.py#L40-L51)).
These are relative to the working directory or the clone, not the home,
so they have no catalog line; `workspace` is also too generic a name to
anchor a glob on. The git tool reads, but does not write,
`~/.git-credentials`
([git.py:69-71](https://github.com/FoundationAgents/MetaGPT/blob/11cdf466d042aece04fc6cfd13b28e1a70341b1f/metagpt/tools/libs/git.py#L69-L71)).

#### Docker

No named volume.

### Credentials

- `.metagpt/config2.yaml` and `.metagpt/config2.bak`: flagged.

### Exclusions

None.

### Project-local files

The `workspace/` tree in [Workspace](#workspace), under no MetaGPT-specific name.

### Where the project path is recorded

Nowhere under the home.

## Parsing

MetaGPT is `collected` only: the collector gathers its configuration, and there is nothing to
parse under the home. The team state and the run logs are written under `METAGPT_ROOT`, under no
MetaGPT-specific name ([Workspace](#workspace)), so no catalog line reaches them and a parser
would have no collected file to read.

## Confidence

- High for every path (source).
- Not determined: the workspace location when the working directory is the home (then
  `~/workspace`, too generic for a glob).
