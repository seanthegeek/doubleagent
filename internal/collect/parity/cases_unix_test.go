//go:build parity && unix

package parity

// skipTar names why the tar fallback cases are not run: they patch the
// collector or the PATH, and the binary has no archiver fallbacks.
const skipTar = "needs a patched copy of the collector with a tar shim first on PATH, to force the " +
	"tar | gzip or plain tar fallback; the binary has no archiver fallbacks"

// parityCases lists every case, in the order of the smoke test.
func parityCases() []parityCase {
	cases := []parityCase{
		{name: "default"},
		{name: "root486", root: "{WORK}/r[1]", args: []string{"--no-docker", "--no-projects"}, setup: setupPatternRoot},
		{name: "linkproj497", args: []string{"-p", "{ROOT}/home/dot/.claude/proj"}, after: checkLinkedSourceIntact,
			expect: linkedProject.expect},
	}
	cases = append(cases, projectLinkCases()...)
	cases = append(cases, []parityCase{
		{name: "nosecrets", args: []string{"--no-secrets"}},
		{name: "envscan", args: []string{"--env-scan-home"}},
		{name: "envscanonce", args: []string{"--env-scan-home"}, setup: setupEnvScanOnce},
		{name: "envscanout", args: []string{"--env-scan-home"}, outDir: "{ALICE}"},
		{name: "envscanglob", args: []string{"--env-scan-home"}, outDir: "{BOB}/out [1]"},
		{name: "envscanroot", args: []string{"--env-scan-home", "-p", "{ROOT}/home", "-p", "{ROOT}/opt/daemonproj"}},
		{name: "full", args: []string{"--full"}},
		{name: "users", args: []string{"-u", "bob"}},
		{name: "nodocker", args: []string{"--no-docker"}},
		{name: "cwd105", cwd: "{WORK}/cwd105", args: []string{"--no-docker", "--no-projects"}},
		{name: "claims106", args: []string{"--no-projects"},
			env: []string{"NESTED_CLAIMS={ROOT}/var/lib/docker/volumes/a0_usr/_data/chats"}},
		{name: "dockerperm", setup: setupUnreadableVolume},
		{name: "desktop", setup: setupDockerDesktop},
		{name: "fwvolumes", setup: setupFrameworkVolumes},
		{name: "fwvolumesnosecrets", args: []string{"--no-secrets"}, setup: setupFrameworkVolumes},
		{name: "tgz31", skip: skipTar},
		{name: "tar31", skip: skipTar},
		{name: "tarfail40", skip: skipTar},
		{name: "projargs", cwd: "{ROOT}/srv", args: []string{"-p", "relproj", "-p", "/nonexistent-cac/proj"},
			setup: setupRelativeProject, expect: outsideArgument{path: "/nonexistent-cac/proj",
				after: "srv/relproj"}.expect},
		{name: "readperm", setup: setupUnreadableHome},
		{name: "cwd109", cwd: "{WORK}/cwd109", args: []string{"--no-docker", "--no-projects"}},
		{name: "cdpath110", cwd: "{WORK}", root: "root", env: []string{"CDPATH={WORK}/cdp110"},
			args: []string{"--no-docker", "--no-projects"}},
		{name: "fold32", setup: setupUnreadableCaseVariant},
		{name: "batch71", root: "{WORK}/root71", args: []string{"--no-docker"}, setup: setupBatchRoot},
		{name: "nobatch71", skip: "CAC_NO_BATCH selects the sh collector's per-file stage path, " +
			"which the binary does not have"},
		{name: "shasum71", skip: "needs a patched copy of the collector whose sha256sum probe fails, " +
			"to force shasum; the binary hashes in Go"},
		{name: "nostat480", skip: "needs a stat shim on PATH and a patched copy of the collector; " +
			"the binary has no stat fallback"},
	}...)
	return append(append(cases, packageCases()...), inventoryCases()...)
}

// packageCases are the three runs over the Claude Desktop MSIX image.
func packageCases() []parityCase {
	return []parityCase{
		{name: "store73", root: "{WORK}/root73", args: []string{"--no-docker"}, setup: setupMsixRoot},
		{name: "nosecrets73", root: "{WORK}/root73", args: []string{"--no-docker", "--no-secrets"},
			setup: setupMsixRoot},
		{name: "full73", root: "{WORK}/root73", args: []string{"--no-docker", "--full"}, setup: setupMsixRoot},
	}
}

// inventoryCases are the stat-only runs: no -o and no staging, one JSON line
// per agent on stdout.
func inventoryCases() []parityCase {
	return []parityCase{
		{name: "inventory", inventory: true, cwd: "{OUT}", args: []string{"-q"}, setup: setupInventory},
		{name: "inventory-o", inventory: true, stderrLog: true, args: []string{"-o", "{OUT}/inv-o"},
			setup: setupInventory},
		{name: "inventory-full", inventory: true, args: []string{"-q", "--full"}, setup: setupInventory},
		{name: "inventory-u", inventory: true, args: []string{"-q", "-u", "bob", "--no-docker"},
			setup: setupInventory},
		{name: "inventory-perm", inventory: true, args: []string{"-q"}, setup: setupUnreadableInventoryHome},
	}
}
