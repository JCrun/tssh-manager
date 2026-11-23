package sshconf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixture = `# Terminal defaults
Host *
    SetEnv TERM=xterm-256color

# LAN
Host pve
    HostName 192.168.0.163
    User root
    #!! Password secret1

Host nas
    HostName 10.0.0.154
    User admin

# VPS
Host bwh
    HostName 80.0.0.1
    Port 29870
    User root
    IdentityFile ~/.ssh/k/id_ed25519

Host jp
    HostName 69.0.0.27
    Port 18977
    User root
    IdentityFile ~/.ssh/k/jp_ed25519
    IdentitiesOnly yes
`

func mustParse(t *testing.T, s string) *Doc {
	t.Helper()
	d, err := Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestRoundTrip(t *testing.T) {
	for _, s := range []string{fixture, strings.TrimSuffix(fixture, "\n"), "", "Host a\n  HostName x # c\n\n\n"} {
		if got := mustParse(t, s).String(); got != s {
			t.Fatalf("round trip changed text:\n%q\n%q", s, got)
		}
	}
}

func TestRealConfigRoundTrip(t *testing.T) {
	if os.Getenv("TSSHM_REAL") == "" {
		t.Skip("set TSSHM_REAL=1 to round-trip ~/.ssh/config (read-only)")
	}
	home, _ := os.UserHomeDir()
	b, err := os.ReadFile(filepath.Join(home, ".ssh", "config"))
	if err != nil {
		t.Fatal(err)
	}
	d := mustParse(t, string(b))
	if d.String() != string(b) {
		t.Fatal("real config does not round-trip")
	}
	t.Logf("%d blocks, sections %v", len(d.Entries()), d.Sections())
}

func TestEntriesAndSections(t *testing.T) {
	d := mustParse(t, fixture)
	es := d.Entries()
	if len(es) != 5 {
		t.Fatalf("want 5 entries, got %d", len(es))
	}
	want := map[string]string{"*": "Terminal defaults", "pve": "LAN", "nas": "LAN", "bwh": "VPS", "jp": "VPS"}
	for _, e := range es {
		if e.Section != want[e.Alias] {
			t.Errorf("%s: section %q, want %q", e.Alias, e.Section, want[e.Alias])
		}
	}
	if !es[0].Wildcard || es[1].Wildcard {
		t.Error("wildcard detection wrong")
	}
	pve := es[1]
	if pve.Effective["HostName"] != "192.168.0.163" || pve.Effective["User"] != "root" {
		t.Errorf("effective: %v", pve.Effective)
	}
	var pw bool
	for _, o := range pve.Options {
		if o.Ext && o.Key == "Password" && o.Value == "secret1" {
			pw = true
		}
	}
	if !pw {
		t.Errorf("#!! Password not read: %+v", pve.Options)
	}
	if got := strings.Join(d.Sections(), ","); got != "Terminal defaults,LAN,VPS" {
		t.Errorf("sections %q", got)
	}
}

func TestApplyKeepsOtherLines(t *testing.T) {
	d := mustParse(t, fixture)
	err := d.Apply("bwh", []Change{
		{Key: "Port", Value: "22"},
		{Key: "IdentitiesOnly", Value: "yes"},
		{Key: "GroupLabels", Value: "us vps", Ext: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(fixture, "    Port 29870\n    User root\n    IdentityFile ~/.ssh/k/id_ed25519\n",
		"    Port 22\n    User root\n    IdentityFile ~/.ssh/k/id_ed25519\n    IdentitiesOnly yes\n    #!! GroupLabels us vps\n", 1)
	if got := d.String(); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	// delete a #!! line and a normal line
	if err := d.Apply("pve", []Change{{Key: "Password", Ext: true}, {Key: "User"}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(d.String(), "secret1") || strings.Contains(d.String(), "    User root\n    #!!") {
		t.Fatalf("delete failed:\n%s", d.String())
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyCaseAndSeparators(t *testing.T) {
	d := mustParse(t, "Host a\n\thostname=old\n")
	if err := d.Apply("a", []Change{{Key: "HostName", Value: "new"}}); err != nil {
		t.Fatal(err)
	}
	if got := d.String(); got != "Host a\n\thostname new\n" {
		t.Fatalf("%q", got)
	}
}

func TestApplyRejectsBadInput(t *testing.T) {
	d := mustParse(t, fixture)
	for _, c := range []Change{{Key: "Host", Value: "x"}, {Key: "Port", Value: "1\nHost evil"}, {Key: "bad key", Value: "x"}, {Key: "Include", Value: "/etc/x"}} {
		if err := d.Apply("bwh", []Change{c}); err == nil {
			t.Errorf("accepted %+v", c)
		}
	}
	if d.String() != fixture {
		t.Fatal("document changed after rejected input")
	}
	if err := d.Apply("missing", nil); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestAddIntoSectionAndNew(t *testing.T) {
	d := mustParse(t, fixture)
	if err := d.Add("nuc", "LAN", []Change{{Key: "HostName", Value: "10.0.0.9"}, {Key: "User", Value: "me"}}); err != nil {
		t.Fatal(err)
	}
	s := d.String()
	if !strings.Contains(s, "    User admin\n\nHost nuc\n    HostName 10.0.0.9\n    User me\n\n# VPS\n") {
		t.Fatalf("not inserted after last LAN host:\n%s", s)
	}
	if err := d.Add("ld", "London", []Change{{Key: "HostName", Value: "185.0.0.1"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(d.String(), "    IdentitiesOnly yes\n\n# London\nHost ld\n    HostName 185.0.0.1\n") {
		t.Fatalf("new section not appended:\n%s", d.String())
	}
	for _, e := range d.Entries() {
		if e.Alias == "ld" && e.Section != "London" || e.Alias == "nuc" && e.Section != "LAN" {
			t.Errorf("%s in section %q", e.Alias, e.Section)
		}
	}
	if err := d.Add("pve", "", nil); err == nil {
		t.Error("duplicate alias accepted")
	}
	if err := d.Add("a b", "", nil); err == nil {
		t.Error("alias with space accepted")
	}
	if err := d.Add("x*", "", nil); err == nil {
		t.Error("wildcard alias accepted")
	}
}

func TestAddToEmpty(t *testing.T) {
	d := mustParse(t, "")
	if err := d.Add("a", "", []Change{{Key: "HostName", Value: "h"}}); err != nil {
		t.Fatal(err)
	}
	if got := d.String(); got != "Host a\n    HostName h\n" {
		t.Fatalf("%q", got)
	}
}

func TestDeleteKeepsNextHeading(t *testing.T) {
	d := mustParse(t, fixture)
	if err := d.Delete("nas"); err != nil {
		t.Fatal(err)
	}
	s := d.String()
	if strings.Contains(s, "Host nas") || !strings.Contains(s, "    #!! Password secret1\n\n# VPS\nHost bwh") {
		t.Fatalf("delete broke layout:\n%s", s)
	}
	if err := d.Delete("jp"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(d.String(), "    IdentityFile ~/.ssh/k/id_ed25519\n") {
		t.Fatalf("delete last:\n%q", d.String())
	}
}

func TestRenameAndRaw(t *testing.T) {
	d := mustParse(t, fixture)
	if err := d.Rename("jp", "bv-jp"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.String(), "\nHost bv-jp\n") {
		t.Fatal("rename failed")
	}
	if err := d.Rename("bv-jp", "bwh"); err == nil {
		t.Error("rename onto existing accepted")
	}
	raw := "Host bwh\n    HostName 80.0.0.2\n    #!! Comment new box"
	if err := d.ReplaceRaw("bwh", raw); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.String(), "# VPS\nHost bwh\n    HostName 80.0.0.2\n    #!! Comment new box\n\nHost bv-jp") {
		t.Fatalf("raw replace:\n%s", d.String())
	}
	for _, bad := range []string{"HostName x", "Host a\nHost b", "Host a\n  HostName x\n# trailing", "Host bv-jp\n  HostName x"} {
		if err := d.ReplaceRaw("bwh", bad); err == nil {
			t.Errorf("raw accepted %q", bad)
		}
	}
}

func TestStoreSaveBackupAndConflict(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config")
	if err := os.WriteFile(p, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(p, filepath.Join(dir, "bak"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Apply("jp", []Change{{Key: "Port", Value: "22"}}); err != nil {
		t.Fatal(err)
	}
	bak, err := s.Save(d)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(bak); string(b) != fixture {
		t.Fatal("backup content wrong")
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode().Perm())
	}
	// outside edit -> conflict
	os.WriteFile(p, []byte(fixture+"\nHost z\n"), 0o600)
	if _, err := s.Save(d); err != ErrChanged {
		t.Fatalf("want ErrChanged, got %v", err)
	}
	// ssh -G rejects an unknown option
	d2, _ := s.Load()
	d2.Apply("jp", []Change{{Key: "NoSuchOption", Value: "1"}})
	if _, err := s.Save(d2); err == nil || !strings.Contains(err.Error(), "ssh rejected") {
		t.Fatalf("want ssh rejection, got %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".config.tsshm*")); len(left) != 0 {
		t.Fatalf("temp files left: %v", left)
	}
}
