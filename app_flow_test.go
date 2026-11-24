package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tssh-manager/internal/settings"
	"tssh-manager/internal/sshconf"
)

// End-to-end through the bound methods, on a temporary config.
func TestAppFlow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	cfg := filepath.Join(dir, "config")
	orig := "Host *\n    SetEnv TERM=xterm-256color\n\n# VPS\nHost a\n    HostName 1.1.1.1\n    User root\n    #!! Password hunter2\n"
	os.WriteFile(cfg, []byte(orig), 0o600)
	fake := filepath.Join(dir, "tssh") // CI runners have no tssh
	os.WriteFile(fake, []byte("#!/bin/sh\n"), 0o755)
	a := &App{ctx: context.Background(), set: settings.Settings{ConfigPath: cfg, KeyDir: filepath.Join(dir, "keys"), TSSH: fake, Terminal: "true tssh {host}"}}

	st, err := a.Load()
	if err != nil || len(st.Hosts) != 2 || st.Hosts[1].Section != "VPS" {
		t.Fatalf("load: %v %+v", err, st)
	}
	// form edit: port + label, rename
	if _, err := a.SaveHost(SaveRequest{OrigAlias: "a", Alias: "a2", Changes: []sshconf.Change{{Key: "Port", Value: "2222"}, {Key: "GroupLabels", Value: "us", Ext: true}}}); err != nil {
		t.Fatal(err)
	}
	// form may never set secrets
	if _, err := a.SaveHost(SaveRequest{OrigAlias: "a2", Alias: "a2", Changes: []sshconf.Change{{Key: "Password", Value: "x", Ext: true}}}); err == nil {
		t.Fatal("form accepted a password")
	}
	// raw edit keeps the masked password
	st, _ = a.Load()
	raw := strings.Replace(st.Hosts[1].Raw, "User root", "User admin", 1)
	if _, err := a.SaveRaw("a2", raw); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(cfg)
	want := "Host *\n    SetEnv TERM=xterm-256color\n\n# VPS\nHost a2\n    HostName 1.1.1.1\n    User admin\n    #!! Password hunter2\n    Port 2222\n    #!! GroupLabels us\n"
	if string(b) != want {
		t.Fatalf("file:\n%s\nwant:\n%s", b, want)
	}
	// new host into a new section, then delete it
	if _, err := a.SaveHost(SaveRequest{Alias: "ld", Section: "London", Changes: []sshconf.Change{{Key: "HostName", Value: "185.0.0.1"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DeleteHost("ld"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RemovePassword("a2"); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(cfg)
	if strings.Contains(string(b), "hunter2") || strings.Contains(string(b), "Host ld") {
		t.Fatalf("after delete/remove:\n%s", b)
	}
	// backups exist and restore the original
	bs, _ := a.ListBackups()
	if len(bs) < 5 {
		t.Fatalf("backups: %d", len(bs))
	}
	if _, err := a.RestoreBackup(bs[len(bs)-1].Path); err != nil {
		t.Fatal(err)
	}
	if b, _ = os.ReadFile(cfg); string(b) != orig {
		t.Fatalf("restore:\n%s", b)
	}
	if _, err := a.RestoreBackup(cfg); err == nil {
		t.Fatal("restored from outside the backup dir")
	}
	// key generation
	k, err := a.GenerateKey("a")
	if err != nil || !k.Created || !strings.HasPrefix(k.Public, "ssh-ed25519 ") {
		t.Fatalf("key: %v %+v", err, k)
	}
	if k2, _ := a.GenerateKey("a"); k2.Created {
		t.Fatal("existing key overwritten")
	}
	// terminal argv: {host} is a separate argument, never a shell string
	argv, _ := a.terminalArgv("a; rm -rf ~", nil)
	if len(argv) != 3 || argv[0] != "true" || argv[1] != fake || argv[2] != "a; rm -rf ~" {
		t.Fatalf("argv %q", argv)
	}
	// Terminal.app: quoted for the shell, then for AppleScript
	m := macTerminal([]string{settings.MacTerminalApp, "/opt/tssh", `it's "x"`})
	if len(m) != 5 || m[0] != "osascript" || m[2] != `tell application "Terminal" to do script "'/opt/tssh' 'it'\\''s \"x\"'"` {
		t.Fatalf("mac %q", m)
	}
	if err := a.Connect("a; rm"); err == nil {
		t.Fatal("Connect accepted an unsafe alias")
	}
	// a missing tssh is reported with install instructions, not a raw exec error
	a.set.TSSH = filepath.Join(dir, "no-such-tssh")
	if _, err := a.terminalArgv("a", nil); err == nil || !strings.Contains(err.Error(), "找不到") {
		t.Fatalf("missing tssh: %v", err)
	}
	if st, _ := a.Load(); len(st.Warnings) == 0 {
		t.Fatal("no warning for missing tssh")
	}
}

func TestPasswordFlow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	cfg := filepath.Join(dir, "config")
	os.WriteFile(cfg, []byte("Host a\n    HostName 1.1.1.1\n\nHost b\n    HostName 2.2.2.2\n    #!! encPassword 00ff\n"), 0o600)
	a := &App{ctx: context.Background(), set: settings.Settings{ConfigPath: cfg, TSSH: "tssh", Terminal: "true tssh {host}"}}

	if _, err := a.RevealPassword("a"); err == nil {
		t.Fatal("reveal without a password should fail")
	}
	if _, err := a.SetPassword("a", "with space"); err != nil {
		t.Fatal(err)
	}
	if pw, err := a.RevealPassword("a"); err != nil || pw != "with space" {
		t.Fatalf("reveal: %q %v", pw, err)
	}
	if _, err := a.SetPassword("a", " lead"); err != nil {
		t.Fatal(err)
	}
	if pw, _ := a.RevealPassword("a"); pw != " lead" {
		t.Fatalf("quoted form: %q", pw)
	}
	if _, err := a.SetPassword("a", "x#y"); err == nil {
		t.Fatal("password with # accepted")
	}
	if _, err := a.SetPassword("a", "line\nbreak"); err == nil {
		t.Fatal("password with newline accepted")
	}
	// the encrypted password on b is not plaintext: no reveal, but it is flagged
	if _, err := a.RevealPassword("b"); err == nil {
		t.Fatal("encPassword revealed as plaintext")
	}
	st, _ := a.Load()
	if !st.Hosts[1].EncPassword || st.Hosts[1].PlainPassword {
		t.Fatalf("b flags: %+v", st.Hosts[1])
	}
	if _, err := a.SetPassword("a", ""); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(cfg)
	if strings.Contains(string(b), "Password lead") || strings.Contains(string(b), `" lead"`) {
		t.Fatalf("password not removed:\n%s", b)
	}
}
