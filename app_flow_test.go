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
	a := &App{ctx: context.Background(), set: settings.Settings{ConfigPath: cfg, KeyDir: filepath.Join(dir, "keys"), TSSH: "tssh", Terminal: "true tssh {host}"}}

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
	if strings.Join(argv, "|") != "true|tssh|a; rm -rf ~" {
		t.Fatalf("argv %q", argv)
	}
	if err := a.Connect("a; rm"); err == nil {
		t.Fatal("Connect accepted an unsafe alias")
	}
}
