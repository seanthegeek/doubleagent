//go:build unix

package volumes

// wantNotes are the notes of the mixed image, with the image root as <root>.
const wantNotes = `docker: <root>/var/lib/containers/storage/volumes is a symlink and was not followed; collect its target by hand
docker: Docker Desktop data at <root>/ProgramData/DockerDesktop; volumes inside its virtual machine disk are not collected
docker: Docker Desktop data at <root>/home/alice/Library/Containers/com.docker.docker; volumes inside its virtual machine disk are not collected`

// wantLog is the run log of the mixed image, with the image root as <root>.
const wantLog = `Docker volumes in <root>/var/lib/docker/volumes
  [agent-zero] <root>/var/lib/docker/volumes/a0_usr/_data
  volume langflow-data has no _data directory, skipped
  volume n8n_data has no _data directory, skipped
  [ollama] <root>/var/lib/docker/volumes/ollama_data/_data
NOTE: docker: <root>/var/lib/containers/storage/volumes is a symlink and was not followed; collect its target by hand
Docker volumes in <root>/home/alice/.local/share/docker/volumes
  [hermes] <root>/home/alice/.local/share/docker/volumes/hermes1/_data
NOTE: docker: Docker Desktop data at <root>/ProgramData/DockerDesktop; volumes inside its virtual machine disk are not collected
NOTE: docker: Docker Desktop data at <root>/home/alice/Library/Containers/com.docker.docker; volumes inside its virtual machine disk are not collected`

// wantRows are the manifest rows of the mixed image, with the image root as
// <root>, the times and owner ids as N, the directory sizes as D and the rows
// of each volume sorted (see normaliseRows).
const wantRows = `{"user":"docker","home":"<root>/var/lib/docker/volumes/a0_usr/_data","agent":"agent-zero","path":"<root>/var/lib/docker/volumes/a0_usr/_data/secrets.env","archive_path":"fs/var/lib/docker/volumes/a0_usr/_data/secrets.env","type":"file","size":5,"mtime":N,"atime":N,"ctime":N,"btime":N,"uid":N,"gid":N,"mode":"644","sha256":"ff7accc0f2ab87bb171e59eff4738352966ee35dbad6b9a775d3de464e6192fc","secret":true,"status":"collected","target":"","error":""}
{"user":"docker","home":"<root>/var/lib/docker/volumes/a0_usr/_data","agent":"agent-zero","path":"<root>/var/lib/docker/volumes/a0_usr/_data/tmp/playwright","archive_path":"","type":"dir","size":D,"mtime":0,"atime":N,"ctime":N,"btime":N,"uid":N,"gid":N,"mode":"0","sha256":"","secret":false,"status":"skipped_excluded","target":"","error":""}
{"user":"docker","home":"<root>/var/lib/docker/volumes/a0_usr/_data","agent":"agent-zero","path":"<root>/var/lib/docker/volumes/a0_usr/_data/x/file","archive_path":"fs/var/lib/docker/volumes/a0_usr/_data/x/file","type":"file","size":2,"mtime":N,"atime":N,"ctime":N,"btime":N,"uid":N,"gid":N,"mode":"644","sha256":"8f434346648f6b96df89dda901c5176b10a6d83961dd3c1ac88b59b2dc327aa4","secret":false,"status":"collected","target":"","error":""}
{"user":"docker","home":"<root>/var/lib/docker/volumes/ollama_data/_data","agent":"ollama","path":"<root>/var/lib/docker/volumes/ollama_data/_data/models","archive_path":"","type":"dir","size":D,"mtime":0,"atime":N,"ctime":N,"btime":N,"uid":N,"gid":N,"mode":"0","sha256":"","secret":false,"status":"skipped_excluded","target":"","error":""}
{"user":"docker","home":"<root>/var/lib/docker/volumes/postgres/_data","agent":"","path":"<root>/var/lib/docker/volumes/postgres/_data","archive_path":"","type":"dir","size":D,"mtime":0,"atime":N,"ctime":N,"btime":N,"uid":N,"gid":N,"mode":"0","sha256":"","secret":false,"status":"skipped_unmatched_volume","target":"","error":""}
{"user":"docker","home":"<root>/home/alice/.local/share/docker/volumes/hermes1/_data","agent":"hermes","path":"<root>/home/alice/.local/share/docker/volumes/hermes1/_data/f","archive_path":"fs/home/alice/.local/share/docker/volumes/hermes1/_data/f","type":"file","size":1,"mtime":N,"atime":N,"ctime":N,"btime":N,"uid":N,"gid":N,"mode":"644","sha256":"2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881","secret":false,"status":"collected","target":"","error":""}`
