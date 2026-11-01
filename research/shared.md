# shared

The catalog agent `shared` is a pseudo-agent for files and directories read by several agents
at once, so they cannot be attributed to one tool: cross-agent instruction and skill
directories. It is collected only, with no parser: these are instruction and skill files, not
transcripts.

## Source and evidence

Evidence level: the per-agent documents in this directory, which cite each reader. Each path
below is justified by at least two agents' source. `AGENTS.md` is read by Codex CLI, Amp,
OpenCode, Crush, Factory Droid, Kilo Code and Copilot CLI (see their
documents). `~/.agents` and `~/.config/agents` hold cross-tool skill and
agent definitions adopted by Codex, Copilot CLI and others.

### Observed

- No real install: `shared` is not one agent, and the evidence is each reader's document.

### Source reviews

- 2026-10-19: the Zed review at
  [`9cd14da`](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6)
  and the Crush review at
  [`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da)
  added both as readers of the skills directories ([Per-user storage](#per-user-storage)).

## Collection

### Per-user storage

| Path relative to home | Holds | Readers |
| --- | --- | --- |
| `.agents` | skills and agent definitions in the cross-tool "agents" layout (`skills/<name>/SKILL.md`) | Codex CLI, Copilot CLI, Amp, Zed, Crush, others |
| `.config/agents` | XDG variant of the same | Codex CLI, Copilot CLI, Amp, Crush, others |
| `.config/AGENTS.md` | user-level instructions | Codex CLI, Amp, OpenCode |

The layout is identical on every OS because the readers use `homedir()`
without platform branching.

Zed reads `~/.agents/skills/<skill>/SKILL.md`
([`crates/agent/src/agent.rs:616-638`](https://github.com/zed-industries/zed/blob/9cd14daad768fbfaeeac8789f515f9a5f67669a6/crates/agent/src/agent.rs#L616-L638)
at
[`9cd14da`](https://github.com/zed-industries/zed/commit/9cd14daad768fbfaeeac8789f515f9a5f67669a6);
[Per-user storage in `research/zed.md`](zed.md#per-user-storage)). Crush reads `~/.config/agents/skills`
and `~/.agents/skills`, and also Claude Code's `~/.claude/skills`
([`internal/config/load.go:1312-1322`](https://github.com/charmbracelet/crush/blob/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da/internal/config/load.go#L1312-L1322)
at
[`7f799fb`](https://github.com/charmbracelet/crush/commit/7f799fbc1fdaf0a931a4ff73e6539bafee59d3da);
[Per-user storage in `research/crush.md`](crush.md#per-user-storage) and
[Per-user storage in `research/claude-code.md`](claude-code.md#per-user-storage)).

Qwen Code lists `~/.agents/skills` beside its own `~/.qwen/skills`
([`packages/core/src/config/storage.ts:17`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L17)
and
[`754-761`](https://github.com/QwenLM/qwen-code/blob/835b080f8567d8612effa248e1fafa11af0e244b/packages/core/src/config/storage.ts#L754-L761)
at
[`835b080`](https://github.com/QwenLM/qwen-code/commit/835b080f8567d8612effa248e1fafa11af0e244b)),
and pi treats it as a trusted user skills directory
([`packages/coding-agent/src/core/trust-manager.ts:190-195`](https://github.com/earendil-works/pi/blob/2c8d1feb147f6b3d97e855da803bfe7b19f3c403/packages/coding-agent/src/core/trust-manager.ts#L190-L195)
at
[`2c8d1fe`](https://github.com/earendil-works/pi/commit/2c8d1feb147f6b3d97e855da803bfe7b19f3c403)).

#### Scratch directories

None known.

### Credentials

None. The home-level and project-level `.env` files that Aider, Gemini
CLI, Qwen Code and Continue read for API keys are general-purpose
environment files and are not collected (collector 1.7.0); they are left to
the EDR. Agent-specific `.env` files inside an agent's own directory, such
as `.gemini/.env` and `.qwen/.env`, are collected under that agent.

### Exclusions

None; these are small text files.

### Project-local files

`AGENTS.md` at the project root, collected through the project catalog.

### Where the project path is recorded

Not applicable.

## Parsing

`shared` is `collected` only: there is nothing to parse. Its paths are cross-agent instruction
and skill directories (`.agents`, `.config/agents`, `.config/AGENTS.md`) with no transcripts,
written by the user and read by several agents, so they record what an agent was told, not what
it did. The transcripts of each reader are parsed, where they are, under that agent.

## Confidence

- High for every path (each reader's source).
- Attribution is the one judgement: a `shared` match says several agents may have read the file,
  not which one did.
