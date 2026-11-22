# Privileges

Run as root (sh) or from an elevated PowerShell (Windows) to collect every
user. A responder does not always have that, and a run without it is still
useful: it collects everything the account can read, finishes, exits `0`,
and records each home, directory and file it could not read as a manifest
row, so the gaps in the archive are visible instead of silent. The log
gets a `WARNING: not running as root` (or `as Administrator`) line, and:

- A home the responder cannot list or enter is still listed in `users`,
  gets one `error_read` row (`agent` empty, `path` and `home` the home
  directory), and is logged as `read failed: <home>: <error>`. The
  catalog is still tried against it, so a home that can be entered but
  not listed (mode `711`) still yields the entries that are not globs.
  Its `error_read` row does not put the user in `users_with_artifacts`.
  A discovered project directory or a `-p` directory that cannot be read
  gets the same row.
- Inside a walk, a directory that cannot be listed or entered gets one
  `error_read` row under the agent being walked, and a `read failed:` log
  line. sh: every directory `find` reaches is tested with `test -r` and
  `test -x` in the worker, and `error` is the first line `ls` printed for
  the same access; `find` still logs its own `find:` line for the
  directory. The
  PowerShell collector writes the row when `Get-ChildItem` fails on the
  directory, with the exception message as `error`. On Linux and macOS a
  directory that can be listed but not entered fails that call only when
  it has entries, so an empty one gets a row from the sh collector alone.
  A catalog path whose parent cannot be entered is not seen by either
  collector and gets no row; the PowerShell collector logs a
  `stat failed:` line, with no row, for a matched path whose metadata it
  cannot read. A file that can be listed but not read is an `error_copy`
  row.
- The root-owned Docker and Podman data roots are reported as unreadable
  (see [docker.md](docker.md#unreadable-volumes)).

On macOS, root is not always enough. Transparency, Consent and Control
(TCC), the macOS privacy system, refuses some directories to any process
that has not been given Full Disk Access in System Settings, root
included, and a root shell reached over ssh does not have it. On a live
macOS 26.6 host the sh collector run that way as root (October 2026)
wrote three `error_read` rows with `Operation not permitted` in `error`
and `agent` empty, one each for `/var/db/lockdown`,
`/var/db/appinstalld` and `/var/db/installcoordinationd`. These are the
home directories of macOS service accounts, which the Users step lists
from the directory service (`dscl`) and tries as homes like any other;
they hold no agent state, so the three rows are expected and harmless.
The user homes on that host were collected with no error rows; a run
from a process with Full Disk Access was not tried.
