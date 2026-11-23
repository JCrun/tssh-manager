package sshconf

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Store owns the on-disk config file.
type Store struct {
	Path      string // resolved path (symlinks followed)
	BackupDir string
	KeepBak   int
	hash      string // sha256 of the content last loaded or saved
}

// ErrChanged means the file was modified outside the app since it was loaded.
var ErrChanged = errors.New("ssh config was changed outside tssh-manager; reload before saving")

func NewStore(path, backupDir string) (*Store, error) {
	if p, err := filepath.EvalSymlinks(path); err == nil {
		path = p
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return &Store{Path: path, BackupDir: backupDir, KeepBak: 30}, nil
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// Load reads and parses the file. A missing file is an empty config.
func (s *Store) Load() (*Doc, error) {
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		b, err = nil, nil
	}
	if err != nil {
		return nil, err
	}
	d, err := Parse(string(b))
	if err != nil {
		return nil, err
	}
	s.hash = sum(b)
	return d, nil
}

// Save validates d, backs up the current file and writes atomically (0600).
func (s *Store) Save(d *Doc) (string, error) {
	if err := d.Validate(); err != nil {
		return "", err
	}
	text := d.String()
	if err := sshCheck(filepath.Dir(s.Path), text); err != nil {
		return "", err
	}
	cur, err := os.ReadFile(s.Path)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if exists && sum(cur) != s.hash {
		return "", ErrChanged
	}
	backup := ""
	if exists {
		if err := os.MkdirAll(s.BackupDir, 0o700); err != nil {
			return "", err
		}
		backup = filepath.Join(s.BackupDir, "config."+time.Now().Format("20060102-150405.000"))
		if err := os.WriteFile(backup, cur, 0o600); err != nil {
			return "", err
		}
		s.prune()
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".config.tsshm-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), s.Path); err != nil {
		return "", err
	}
	s.hash = sum([]byte(text))
	return backup, nil
}

func (s *Store) prune() {
	m, _ := filepath.Glob(filepath.Join(s.BackupDir, "config.*"))
	sort.Strings(m)
	for len(m) > s.KeepBak {
		os.Remove(m[0])
		m = m[1:]
	}
}

// Backups lists backup files, newest first.
func (s *Store) Backups() []string {
	m, _ := filepath.Glob(filepath.Join(s.BackupDir, "config.*"))
	sort.Sort(sort.Reverse(sort.StringSlice(m)))
	return m
}

// sshCheck lets OpenSSH itself parse the candidate file, catching unknown
// options or bad values that the Go parser accepts. "#!!" lines are comments to
// OpenSSH. It is skipped if ssh is not installed. The temporary copy is
// created 0600 next to the real file, because it may hold "#!! Password" lines.
func sshCheck(dir, text string) error {
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".config.tsshm-check-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		return err
	}
	f.Close()
	out, err := exec.Command(ssh, "-G", "-F", f.Name(), "tsshm-validate.invalid").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		msg = strings.ReplaceAll(msg, f.Name(), "config")
		return fmt.Errorf("ssh rejected the config: %s", msg)
	}
	return nil
}
