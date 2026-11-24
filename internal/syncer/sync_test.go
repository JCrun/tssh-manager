package syncer

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

const base0 = `# defaults
Host *
    ServerAliveInterval 30

# LAN
Host pve
    HostName 192.168.0.163
    User root
    #!! Password secret1

Host nas
    HostName 10.0.0.154

# VPS
Host jp
    HostName 69.0.0.27
`

func merge(t *testing.T, b, l, r string, res map[string]string) (string, []Conflict) {
	t.Helper()
	m, c, _, err := mergeConfig(b, l, r, res)
	if err != nil {
		t.Fatal(err)
	}
	return m, c
}

func TestMergeDifferentHosts(t *testing.T) {
	l := strings.Replace(base0, "HostName 10.0.0.154", "HostName 10.0.0.155", 1)                          // local edits nas
	r := strings.Replace(base0, "    HostName 69.0.0.27\n", "    HostName 69.0.0.27\n    Port 2222\n", 1) // remote edits jp
	r += "\nHost new\n    HostName 1.2.3.4\n"                                                             // and adds a host
	m, c := merge(t, base0, l, r, nil)
	if len(c) != 0 {
		t.Fatalf("conflicts: %+v", c)
	}
	want := strings.Replace(r, "HostName 10.0.0.154", "HostName 10.0.0.155", 1)
	if m != want {
		t.Fatalf("got:\n%s\nwant:\n%s", m, want)
	}
}

func TestMergeDeleteAndAddInSection(t *testing.T) {
	// local deletes nas (the section heading "# VPS" stays attached to jp)
	l := strings.Replace(base0, "Host nas\n    HostName 10.0.0.154\n\n", "", 1)
	// remote adds a LAN host after nas and edits jp
	r := strings.Replace(base0, "    HostName 10.0.0.154\n", "    HostName 10.0.0.154\n\nHost nuc\n    HostName 10.0.0.9\n", 1)
	r = strings.Replace(r, "69.0.0.27", "69.0.0.28", 1)
	m, c := merge(t, base0, l, r, nil)
	if len(c) != 0 {
		t.Fatalf("conflicts: %+v", c)
	}
	if strings.Contains(m, "Host nas") || !strings.Contains(m, "Host nuc\n    HostName 10.0.0.9") || !strings.Contains(m, "69.0.0.28") {
		t.Fatalf("merged:\n%s", m)
	}
	if strings.Index(m, "Host nuc") > strings.Index(m, "# VPS") {
		t.Fatalf("nuc not in LAN section:\n%s", m)
	}
}

func TestMergeConflict(t *testing.T) {
	l := strings.Replace(base0, "User root", "User admin", 1)
	r := strings.Replace(base0, "#!! Password secret1", "#!! Password secret2", 1)
	_, c := merge(t, base0, l, r, nil)
	if len(c) != 1 || c[0].ID != "cfg:host:pve" {
		t.Fatalf("conflicts: %+v", c)
	}
	if strings.Contains(c[0].Local+c[0].Remote, "secret") {
		t.Fatalf("secret leaked into conflict: %+v", c[0])
	}
	m, c := merge(t, base0, l, r, map[string]string{"cfg:host:pve": "remote"})
	if len(c) != 0 || m != r {
		t.Fatalf("resolved: %v\n%s", c, m)
	}
	// delete vs edit is a conflict too
	ld := strings.Replace(base0, "Host nas\n    HostName 10.0.0.154\n\n", "", 1)
	re := strings.Replace(base0, "10.0.0.154", "10.0.0.1", 1)
	if _, c := merge(t, base0, ld, re, nil); len(c) != 1 || c[0].Local != "" {
		t.Fatalf("delete/edit: %+v", c)
	}
}

func TestMergeNoBase(t *testing.T) {
	// a second device with its own hosts joins: union, same hosts are fine
	l := "Host pve\n    HostName 192.168.0.163\n    User root\n    #!! Password secret1\n\nHost laptop-only\n    HostName 1.1.1.1\n"
	m, c := merge(t, "", l, base0, nil)
	if len(c) != 0 || !strings.Contains(m, "Host laptop-only") || !strings.Contains(m, "Host jp") {
		t.Fatalf("%v\n%s", c, m)
	}
}

func TestMain(m *testing.M) { workFactor = 10; os.Exit(m.Run()) }

// device simulates one machine.
type device struct{ settings, cfg, keys string }

func newDevice(t *testing.T, root, name, cfg string, c Config) device {
	d := device{filepath.Join(root, name, "settings"), filepath.Join(root, name, "ssh", "config"), filepath.Join(root, name, "keys")}
	os.MkdirAll(filepath.Dir(d.cfg), 0o700)
	if cfg != "" {
		os.WriteFile(d.cfg, []byte(cfg), 0o600)
	}
	c.Device = name
	if err := SaveConfig(d.settings, c); err != nil {
		t.Fatal(err)
	}
	return d
}

func (d device) run(t *testing.T, res map[string]string) Result {
	t.Helper()
	r, err := Run(context.Background(), Options{SettingsDir: d.settings, ConfigPath: d.cfg, KeyDir: d.keys, Resolve: res})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (d device) read(t *testing.T) string { b, _ := os.ReadFile(d.cfg); return string(b) }

func twoDevices(t *testing.T, c Config) {
	root := t.TempDir()
	a := newDevice(t, root, "a", base0, c)
	b := newDevice(t, root, "b", "", c)
	if r := a.run(t, nil); r.Status != "pushed" {
		t.Fatalf("a first: %+v", r)
	}
	if r := b.run(t, nil); r.Status != "pulled" || b.read(t) != base0 {
		t.Fatalf("b first: %+v\n%s", r, b.read(t))
	}
	if r := a.run(t, nil); r.Status != "uptodate" {
		t.Fatalf("a again: %+v", r)
	}
	// both edit different hosts
	os.WriteFile(a.cfg, []byte(strings.Replace(a.read(t), "10.0.0.154", "10.0.0.155", 1)), 0o600)
	os.WriteFile(b.cfg, []byte(b.read(t)+"\nHost b-new\n    HostName 2.2.2.2\n"), 0o600)
	if r := a.run(t, nil); r.Status != "pushed" {
		t.Fatalf("a push: %+v", r)
	}
	if r := b.run(t, nil); r.Status != "merged" || len(r.Changes.FromRemote) != 1 || r.Changes.FromRemote[0] != "nas" {
		t.Fatalf("b merge: %+v", r)
	}
	if r := a.run(t, nil); r.Status != "pulled" || a.read(t) != b.read(t) {
		t.Fatalf("a pull: %+v\n%s\n---\n%s", r, a.read(t), b.read(t))
	}
	// conflict, resolved on b
	os.WriteFile(a.cfg, []byte(strings.Replace(a.read(t), "User root", "User a", 1)), 0o600)
	os.WriteFile(b.cfg, []byte(strings.Replace(b.read(t), "User root", "User b", 1)), 0o600)
	a.run(t, nil)
	r := b.run(t, nil)
	if r.Status != "conflict" || len(r.Conflicts) != 1 || !strings.Contains(b.read(t), "User b") {
		t.Fatalf("b conflict: %+v", r)
	}
	if r := b.run(t, map[string]string{"cfg:host:pve": "local"}); r.Status != "pushed" {
		t.Fatalf("b resolve: %+v", r)
	}
	if a.run(t, nil); !strings.Contains(a.read(t), "User b") {
		t.Fatalf("a after resolve:\n%s", a.read(t))
	}
	// wrong passphrase
	c2 := c
	c2.Passphrase = "wrong-passphrase"
	x := newDevice(t, root, "x", base0, c2)
	if _, err := Run(context.Background(), Options{SettingsDir: x.settings, ConfigPath: x.cfg, KeyDir: x.keys}); err != ErrPassphrase {
		t.Fatalf("wrong passphrase: %v", err)
	}
}

func TestFolderSync(t *testing.T) {
	remote := t.TempDir()
	twoDevices(t, Config{Backend: "folder", Folder: remote, Passphrase: "correct horse battery"})
	b, _ := os.ReadFile(filepath.Join(remote, FileName))
	if !strings.HasPrefix(string(b), "age-encryption.org/v1") || strings.Contains(string(b), "secret1") {
		t.Fatal("remote file is not an age file")
	}
}

// minimal WebDAV server with ETags and Basic auth
func TestWebDAVSync(t *testing.T) {
	var mu sync.Mutex
	files := map[string][]byte{}
	etag := map[string]string{}
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if u, p, ok := r.BasicAuth(); !ok || u != "me" || p != "pw" {
			w.WriteHeader(401)
			return
		}
		cur, exists := files[r.URL.Path]
		switch r.Method {
		case "GET":
			if !exists {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("ETag", etag[r.URL.Path])
			w.Write(cur)
		case "PUT":
			if r.Header.Get("If-None-Match") == "*" && exists || r.Header.Get("If-Match") != "" && r.Header.Get("If-Match") != etag[r.URL.Path] {
				w.WriteHeader(412)
				return
			}
			b, _ := io.ReadAll(r.Body)
			files[r.URL.Path] = b
			n++
			etag[r.URL.Path] = `"v` + string(rune('0'+n)) + `"`
			w.WriteHeader(201)
		}
	}))
	defer srv.Close()
	twoDevices(t, Config{Backend: "webdav", WebDAVURL: srv.URL + "/dav/ssh", WebDAVUser: "me", WebDAVPassword: "pw", Passphrase: "correct horse battery"})
}

func TestKeySync(t *testing.T) {
	root, remote := t.TempDir(), t.TempDir()
	c := Config{Backend: "folder", Folder: remote, Passphrase: "correct horse battery", SyncKeys: true}
	a := newDevice(t, root, "a", base0, c)
	b := newDevice(t, root, "b", "", c)
	os.MkdirAll(a.keys, 0o700)
	os.WriteFile(filepath.Join(a.keys, "jp_ed25519"), []byte("PRIVATE"), 0o600)
	os.WriteFile(filepath.Join(a.keys, "jp_ed25519.pub"), []byte("ssh-ed25519 AAA"), 0o644)
	a.run(t, nil)
	b.run(t, nil)
	got, err := os.ReadFile(filepath.Join(b.keys, "jp_ed25519"))
	st, _ := os.Stat(filepath.Join(b.keys, "jp_ed25519"))
	if err != nil || string(got) != "PRIVATE" || (runtime.GOOS != "windows" && st.Mode().Perm() != 0o600) {
		t.Fatalf("key not synced: %v %q %v", err, got, st.Mode())
	}
	// delete on b moves the file to backups on a, never removes it
	os.Remove(filepath.Join(b.keys, "jp_ed25519.pub"))
	b.run(t, nil)
	a.run(t, nil)
	if _, err := os.Stat(filepath.Join(a.keys, "jp_ed25519.pub")); !os.IsNotExist(err) {
		t.Fatal("deleted key still present on a")
	}
	if m, _ := filepath.Glob(filepath.Join(a.settings, "backups", "keys", "jp_ed25519.pub.*")); len(m) != 1 {
		t.Fatal("deleted key not kept in backups")
	}
}
