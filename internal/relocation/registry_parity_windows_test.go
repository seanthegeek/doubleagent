//go:build parity && windows

package relocation

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestValueNamesMatchWindowsPowerShell(t *testing.T) {
	key, usersPath := testKey(t)
	if err := key.SetStringValue("CODEX_HOME", `C:\real`); err != nil {
		t.Fatal(err)
	}
	setCountedName(t, key, "CODEX_HOME\x00x", `C:\other`)
	subkey := usersPath[strings.IndexByte(usersPath, '\\')+1:]
	// Read-RegistryStrings lists the names through RegistryKey.GetValueNames;
	// each is printed with its NULs spelled <NUL>.
	script := "$key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('" + subkey + "'); " +
		"foreach ($name in $key.GetValueNames()) { " +
		"[Console]::Out.Write($name.Replace([string][char]0, '<NUL>') + \"`n\") }"
	powerShell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0",
		"powershell.exe")
	output, err := exec.Command(powerShell, "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		t.Fatalf("powershell.exe: %v", err)
	}
	want := strings.Fields(string(output))
	var got []string
	for _, name := range SystemRegistry().Strings(UsersRoot, usersPath).Names {
		got = append(got, strings.ReplaceAll(name, "\x00", "<NUL>"))
	}
	t.Logf("Windows PowerShell names %q, binary names %q", want, got)
	if !slices.Equal(got, want) {
		t.Errorf("binary names %q, Windows PowerShell's GetValueNames %q", got, want)
	}
}
