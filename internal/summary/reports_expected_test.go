//go:build unix

package summary

// wantCollectionEveryHome is collection.json of the run.
const wantCollectionEveryHome = `{
  "tool": "doubleagent",
  "version": "1.22.0",
  "hostname": "testhost",
  "mode": "image",
  "root": "/img/root",
  "platform": "Linux 6.8.0 x86_64",
  "run_as_uid": "1000",
  "started": "2026-01-01T00:00:00Z",
  "finished": "2026-01-01T00:00:00Z",
  "options": {
    "full": false,
    "no_secrets": false,
    "max_file_size_bytes": 1048576,
    "users_filter": "",
    "no_docker": false,
    "no_projects": false,
    "projects": [],
    "env_scan_home": false,
    "env_scan_max_size_bytes": 1048576,
    "no_preserve_atime": false
  },
  "capabilities": {
    "hash_tool": "crypto/sha256",
    "archiver": "archive/tar",
    "grep_text": true,
    "plutil": true
  },
  "users": [
    "alice",
    "bob"
  ],
  "homes": [
    "/img/root/home/alice",
    "/img/root/home/bob"
  ],
  "users_with_artifacts": [
    "alice"
  ],
  "projects": [],
  "counts": {
    "collected": 4,
    "symlink": 0,
    "skipped_excluded": 0,
    "skipped_size": 1,
    "skipped_secret": 0,
    "skipped_special": 0,
    "error_copy": 0,
    "skipped_unmatched_volume": 1,
    "error_read": 0,
    "collected_bytes": 79
  },
  "docker": {
    "volumes_found": 2,
    "volumes_collected": 1,
    "unreadable": 0,
    "docker_desktop": true
  },
  "relocation": {
    "hints": [
      {
        "user": "alice",
        "source": "/img/root/home/alice/.bashrc",
        "line": 1,
        "variable": "CLAUDE_CONFIG_DIR",
        "agent": "claude-code",
        "value": ""
      },
      {
        "user": "alice",
        "source": "/img/root/home/alice/.bashrc",
        "line": 2,
        "variable": "CODEX_HOME",
        "agent": "codex-cli",
        "value": ""
      }
    ],
    "registry": {
      "read": false,
      "reason": "sh collector",
      "users_read": [],
      "users_not_loaded": [],
      "users_denied": []
    },
    "files_searched": 2,
    "files_skipped_size": 0
  },
  "notes": [
    "docker: Docker Desktop data at /img/root/home/alice/Library/Containers/com.docker.docker; volumes inside its virtual machine disk are not collected"
  ],
  "archive": "testhost_20260101T000000Z_agent-artifacts.tar.gz"
}
`

// wantResultEveryHome is the summary printed to standard output.
const wantResultEveryHome = `archive:    /out/testhost_20260101T000000Z_agent-artifacts.tar.gz
size:       2140 (2.1 KiB)
sha256:     66dc0374b97dc17499595d87a056311bd4db9523503ce69c6cbadd86633818c7
manifest:   /out/testhost_20260101T000000Z_agent-artifacts.manifest.jsonl
summary:    /out/testhost_20260101T000000Z_agent-artifacts.collection.json
log:        /out/testhost_20260101T000000Z_agent-artifacts.log
users:      1
projects:   0
collected:  4 files, 79 bytes (79 B)
skipped:    0 excluded, 1 too large, 0 secret, 0 special
errors:     0 (0 error_copy, 0 error_read)
relocation: 2 hints in 2 files searched; see collectors/docs/relocation.md
docker:     2 volumes found, 1 collected, 0 unreadable; Docker Desktop VM disk not collected
`

// wantCollectionOneUser is collection.json of the run.
const wantCollectionOneUser = `{
  "tool": "doubleagent",
  "version": "1.22.0",
  "hostname": "testhost",
  "mode": "image",
  "root": "/img/root",
  "platform": "Linux 6.8.0 x86_64",
  "run_as_uid": "1000",
  "started": "2026-01-01T00:00:00Z",
  "finished": "2026-01-01T00:00:01Z",
  "options": {
    "full": true,
    "no_secrets": true,
    "max_file_size_bytes": 0,
    "users_filter": "alice",
    "no_docker": true,
    "no_projects": false,
    "projects": [
      "/img/proj"
    ],
    "env_scan_home": true,
    "env_scan_max_size_bytes": 1048576,
    "no_preserve_atime": false
  },
  "capabilities": {
    "hash_tool": "crypto/sha256",
    "archiver": "archive/tar",
    "grep_text": true,
    "plutil": true
  },
  "users": [
    "alice"
  ],
  "homes": [
    "/img/root/home/alice"
  ],
  "users_with_artifacts": [
    "alice"
  ],
  "projects": [
    "/img/proj"
  ],
  "counts": {
    "collected": 1,
    "symlink": 0,
    "skipped_excluded": 0,
    "skipped_size": 0,
    "skipped_secret": 3,
    "skipped_special": 0,
    "error_copy": 0,
    "skipped_unmatched_volume": 0,
    "error_read": 0,
    "collected_bytes": 2097152
  },
  "docker": {
    "volumes_found": 0,
    "volumes_collected": 0,
    "unreadable": 0,
    "docker_desktop": false
  },
  "relocation": {
    "hints": [
      {
        "user": "alice",
        "source": "/img/root/home/alice/.bashrc",
        "line": 1,
        "variable": "CLAUDE_CONFIG_DIR",
        "agent": "claude-code",
        "value": ""
      },
      {
        "user": "alice",
        "source": "/img/root/home/alice/.bashrc",
        "line": 2,
        "variable": "CODEX_HOME",
        "agent": "codex-cli",
        "value": ""
      }
    ],
    "registry": {
      "read": false,
      "reason": "sh collector",
      "users_read": [],
      "users_not_loaded": [],
      "users_denied": []
    },
    "files_searched": 2,
    "files_skipped_size": 0
  },
  "notes": [],
  "archive": "testhost_20260101T000000Z_agent-artifacts.tar.gz"
}
`

// wantResultOneUser is the summary printed to standard output.
const wantResultOneUser = `archive:    /out/testhost_20260101T000000Z_agent-artifacts.tar.gz
size:       3787 (3.7 KiB)
sha256:     b8f654d2c474215358e4efd01d3beb04a59b362e62f8c0f5b24a27d79886bbfc
manifest:   /out/testhost_20260101T000000Z_agent-artifacts.manifest.jsonl
summary:    /out/testhost_20260101T000000Z_agent-artifacts.collection.json
log:        /out/testhost_20260101T000000Z_agent-artifacts.log
users:      1
projects:   1
collected:  1 file, 2097152 bytes (2.0 MiB)
skipped:    0 excluded, 0 too large, 3 secret, 0 special
errors:     0 (0 error_copy, 0 error_read)
relocation: 2 hints in 2 files searched; see collectors/docs/relocation.md
`
