# Privileges

Run as root on Linux and macOS, or from a window opened with **Run as
administrator** on Windows, to collect every user. A responder does not
always have that, and a run without it is still useful: it collects
everything the account can read, finishes, exits `0`, and records each
home, directory and file it could not read as a manifest row, so the gaps
in the archive are visible rather than silent. A live run without the
privilege logs, among its start lines,
`WARNING: not running as root; other users' homes will probably be unreadable`
(on Windows, `WARNING: not running as Administrator; other users' profiles will probably be unreadable`).
An image-mode run (`-r`) gives no such warning, but reads the image only as
far as the account may.

On Windows "Administrator" means that the account is a member of the
Administrators group with that membership in effect: an administrator's
ordinary window, under User Account Control, does not count.
`collection.json` records the account as `run_as` and `run_as_admin`, and
on Linux and macOS the user ID as `run_as_uid`.

What a run without the privilege records:

- A home the account cannot list or enter is still listed in
  `collection.json` `users`, gets one `error_read` row (`agent` empty,
  `path` and `home` the home directory) and the log line
  `read failed: HOME: REASON`. The catalog is still tried against it, so a
  home that can be entered but not listed (mode `711`) still yields the
  entries that are not patterns. Its `error_read` row does not put the user
  in `users_with_artifacts`. A project directory, or a `-p` directory, that
  cannot be read gets the same row.
- Inside a walk, a directory that cannot be listed or entered gets one
  `error_read` row under the agent being walked, and the log line
  `read failed: DIR: REASON`. Nothing under it is
  collected, and its entries get no row. A catalog path whose parent
  directory cannot be entered is not seen and gets no row.
- A file that can be listed but not read is an `error_copy` row, with the
  log line `copy failed: PATH: REASON`.
- On Windows, a profile the registry lists whose directory the account
  cannot look at is logged `profile not accessible (skipped): PATH`; when
  its parent directory lists it, it also gets an `error_read` row.
- The Docker and Podman data directories owned by root are reported as
  unreadable ([Unreadable volumes](docker.md#unreadable-volumes)).
- On a live Windows host, another user's loaded registry hive can be opened
  only by an administrator: the user is listed in
  `relocation.registry.users_denied` and the relocation hints in that hive
  are not found ([The registry](relocation.md#the-registry)).

The `error` field holds the operating system's reason, such as
`open /home/erin: permission denied` or, on Windows,
`open C:\Users\erin: Access is denied.`

On macOS, root is not always enough. Transparency, Consent and Control
(TCC), the macOS privacy system, refuses some directories to any process
that has not been given Full Disk Access in System Settings, root included,
and a root shell reached over ssh does not have it. Run that way on a live
macOS host, a collection gets `error_read` rows with
`operation not permitted` in `error` and an empty `agent` for the home
directories of some macOS service accounts, such as `/var/db/lockdown`,
`/var/db/appinstalld` and `/var/db/installcoordinationd`. Those homes come
from the directory service (`dscl`) and are tried like any other; they hold
no agent state, so these rows are expected and harmless.
