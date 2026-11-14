//go:build parity && windows

package parity

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// The two live runs of smoke.ps1: collections without -r, which read the
// account the test runs as. They run only where PARITY_LIVE=1, which only
// the disposable CI runner and the lab VM set, and write under t.TempDir(),
// which goes when the test ends.

// liveRun is one live run of both collectors: the arguments after the
// output options, and a check of the binary's run beyond the comparison.
type liveRun struct {
	name  string
	args  []string
	check func(t *testing.T, got collectorRun)
}

func TestLiveRunsAgreeWithThePowerShellCollector(t *testing.T) {
	if os.Getenv("PARITY_LIVE") != "1" {
		t.Skip("live runs collect the account the test runs as; they run only where PARITY_LIVE=1 " +
			"(the disposable CI runner and the lab VM)")
	}
	repository := repositoryRoot(t)
	binary := []string{buildBinary(t, repository), "collect"}
	reference := powerShellCollector(filepath.Join(repository, "collectors", "Collect-AgentArtifacts.ps1"))
	profileUser := filepath.Base(os.Getenv("USERPROFILE"))
	runs := []liveRun{
		{name: "live32", args: []string{"-u", "no-such-user-cac", "--no-docker", "--no-projects"}},
		{name: "registry34", args: []string{"-u", profileUser, "--no-docker", "--no-projects"},
			check: checkRegistryHint(profileUser)},
	}
	for _, run := range runs {
		t.Run(run.name, func(t *testing.T) {
			if run.check != nil {
				setRegistryValue(t)
			}
			want := reference(t, liveInvocation(t, run.args))
			got := execute(t, binary, liveInvocation(t, run.args))
			normalise := newWindowsNormaliser()
			compareRuns(t, normalise(want, false, false), normalise(got, true, false))
			if run.check != nil {
				run.check(t, got)
			}
		})
	}
}

// liveInvocation returns a live run into a new temporary directory.
func liveInvocation(t *testing.T, args []string) invocation {
	t.Helper()
	dir := longPath(t, t.TempDir())
	return invocation{args: append([]string{"-o", dir, "-q"}, args...), outDir: dir}
}

// setRegistryValue sets CODEX_HOME in the user's HKCU\Environment as an
// expandable string naming %USERPROFILE%, and deletes it at the end of the
// case. A value already there is the account's own: the case is skipped.
func setRegistryValue(t *testing.T) {
	t.Helper()
	key, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = key.Close() })
	if _, _, err := key.GetStringValue("CODEX_HOME"); !errors.Is(err, registry.ErrNotExist) {
		t.Skipf("HKCU\\Environment already sets CODEX_HOME (%v); the registry run is skipped", err)
	}
	if err := key.SetExpandStringValue("CODEX_HOME", `%USERPROFILE%\cac-codex`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := key.DeleteValue("CODEX_HOME"); err != nil {
			t.Errorf("delete HKCU\\Environment\\CODEX_HOME: %v", err)
		}
	})
}

// liveSummary is the part of collection.json the registry check reads.
type liveSummary struct {
	Relocation struct {
		Hints []struct {
			User, Agent, Variable, Source, Value string
			Line                                 int
		}
		Registry struct {
			Read      bool
			Reason    string
			UsersRead []string `json:"users_read"`
		}
	}
}

// checkRegistryHint returns the check that the binary read CODEX_HOME from
// the user's hive unexpanded and expanded it from the profile path, and
// lists the user among the hives it read.
func checkRegistryHint(profileUser string) func(t *testing.T, got collectorRun) {
	return func(t *testing.T, got collectorRun) {
		t.Helper()
		var summary liveSummary
		if err := json.Unmarshal([]byte(got.files[".collection.json"]), &summary); err != nil {
			t.Fatalf("collection.json: %v", err)
		}
		source := `HKU\` + currentUser(t) + `\Environment\CODEX_HOME`
		value := os.Getenv("USERPROFILE") + `\cac-codex`
		found := 0
		for _, hint := range summary.Relocation.Hints {
			if hint.Source != source {
				continue
			}
			found++
			if hint.Variable != "CODEX_HOME" || hint.Agent != "codex-cli" || hint.User != profileUser ||
				hint.Line != 0 || hint.Value != value {
				t.Errorf("registry hint %+v, want CODEX_HOME, codex-cli, %s, line 0, %s", hint, profileUser, value)
			}
		}
		if found != 1 {
			t.Errorf("%d hints from %s, want 1", found, source)
		}
		registryRead := summary.Relocation.Registry
		if !registryRead.Read || registryRead.Reason != "" || !slices.Contains(registryRead.UsersRead, profileUser) {
			t.Errorf("relocation.registry %+v, want read, no reason and %s among users_read", registryRead, profileUser)
		}
	}
}
