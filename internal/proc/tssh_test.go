package proc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindTSSH(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	if got := FindTSSH("tssh"); got.Source != "missing" || got.Err() == nil {
		t.Fatalf("empty PATH: %+v", got)
	}
	fake := filepath.Join(dir, exeName("tssh"))
	os.WriteFile(fake, []byte("#!/bin/sh\n"), 0o755)
	if got := FindTSSH("tssh"); got.Source != "path" || got.Path != fake {
		t.Fatalf("on PATH: %+v", got)
	}
	if got := FindTSSH(fake); got.Source != "setting" || got.Err() != nil {
		t.Fatalf("setting: %+v", got)
	}
	if got := FindTSSH(filepath.Join(dir, "nope")); got.Source != "missing" || got.Err() == nil {
		t.Fatalf("bad setting: %+v", got)
	}
	// not on PATH, but shipped next to the executable
	os.Remove(fake)
	exe, _ := os.Executable()
	bundled := filepath.Join(filepath.Dir(exe), exeName("tssh"))
	if err := os.WriteFile(bundled, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Skip(err)
	}
	defer os.Remove(bundled)
	want, _ := filepath.EvalSymlinks(bundled) // macOS: /var -> /private/var
	if got := FindTSSH("tssh"); got.Source != "bundled" || got.Path != want {
		t.Fatalf("bundled: %+v", got)
	}
}
