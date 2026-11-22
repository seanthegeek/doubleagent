# internal/CLAUDE.md

@AGENTS.md

Notes for Claude Code on this workstation:

- Run `export PATH=/usr/local/go/bin:$PATH GOFLAGS=-tags=noasm,nounsafe`
  before a `go` command; the `make` targets need only the `PATH`.
- `DOUBLEAGENT_UPDATE_GOLDEN=1` here rewrites only the macOS view of the
  collection's expected files; the Linux and Windows files come from a run
  on the CI runner or a lab VM.
