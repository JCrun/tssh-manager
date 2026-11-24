// Package syncer keeps ~/.ssh/config (and optionally the key directory) the
// same on several machines through one end-to-end encrypted file on a
// folder or WebDAV remote.
//
// Each device remembers the snapshot it last synced ("base"). A sync merges
// base, local and remote three ways per Host block, writes the result locally
// through sshconf.Store (validated, backed up, atomic) and uploads it only if
// the remote is still at the revision that was read.
package syncer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"tssh-manager/internal/sshconf"
)

// Config is stored in <settings dir>/sync.json (mode 0600). The passphrase
// and WebDAV password sit next to ~/.ssh/config, which already holds tssh
// passwords in plain text; the remote copy is what they protect.
type Config struct {
	Backend        string `json:"backend"` // "" (off) | folder | webdav
	Folder         string `json:"folder"`
	WebDAVURL      string `json:"webdavURL"`
	WebDAVUser     string `json:"webdavUser"`
	WebDAVPassword string `json:"webdavPassword"`
	Passphrase     string `json:"passphrase"`
	SyncKeys       bool   `json:"syncKeys"` // also sync private keys in the key directory
	Auto           bool   `json:"auto"`     // sync at start and after each save
	Device         string `json:"device"`
}

func ConfigPath(dir string) string { return filepath.Join(dir, "sync.json") }
func statePath(dir string) string  { return filepath.Join(dir, "sync-base.json") }

func LoadConfig(dir string) Config {
	var c Config
	if b, err := os.ReadFile(ConfigPath(dir)); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	if c.Device == "" {
		c.Device, _ = os.Hostname()
	}
	if p := os.Getenv("TSSHM_SYNC_PASSPHRASE"); p != "" {
		c.Passphrase = p
	}
	return c
}

func SaveConfig(dir string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	return writeJSON(ConfigPath(dir), c)
}

func (c Config) Validate() error {
	switch c.Backend {
	case "":
		return nil
	case "folder":
		if !filepath.IsAbs(expand(c.Folder)) {
			return errors.New("同步文件夹需要是绝对路径")
		}
	case "webdav":
		if _, err := newWebDAV(c.WebDAVURL, c.WebDAVUser, c.WebDAVPassword); err != nil {
			return err
		}
	default:
		return fmt.Errorf("未知的同步方式 %q", c.Backend)
	}
	if len(c.Passphrase) < 8 {
		return errors.New("同步口令至少 8 个字符（所有设备要用同一个口令）")
	}
	return nil
}

func expand(p string) string {
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

func (c Config) backend() (Backend, error) {
	switch c.Backend {
	case "folder":
		return folder{dir: expand(c.Folder)}, nil
	case "webdav":
		return newWebDAV(c.WebDAVURL, c.WebDAVUser, c.WebDAVPassword)
	}
	return nil, errors.New("同步未开启")
}

func writeJSON(p string, v any) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// payload is what gets encrypted.
type payload struct {
	Format  int               `json:"format"`
	Device  string            `json:"device"`
	Updated int64             `json:"updated"`
	Config  string            `json:"config"`
	Keys    map[string]string `json:"keys,omitempty"` // file name -> base64
}

// snapshot is the last synced state of this device.
type snapshot struct {
	Config   string            `json:"config"`
	Keys     map[string]string `json:"keys"`
	SyncKeys bool              `json:"syncKeys"`
	Time     int64             `json:"time"`
	Device   string            `json:"remoteDevice"`
}

// Options for one run.
type Options struct {
	SettingsDir string            // where sync.json, sync-base.json and backups live
	ConfigPath  string            // ~/.ssh/config
	KeyDir      string            // synced when Config.SyncKeys
	Resolve     map[string]string // Conflict.ID -> "local" | "remote"
}

// Result of one run. Status: uptodate | pulled | pushed | merged | conflict.
type Result struct {
	Status       string     `json:"status"`
	Remote       string     `json:"remote"`
	RemoteDevice string     `json:"remoteDevice"`
	Changes      Changes    `json:"changes"`
	Conflicts    []Conflict `json:"conflicts"`
	Warnings     []string   `json:"warnings"`
	Backup       string     `json:"backup"`
	Time         int64      `json:"time"`
}

// LastSync returns when this device last synced (unix ms, 0 = never).
func LastSync(dir string) int64 {
	var s snapshot
	if b, err := os.ReadFile(statePath(dir)); err == nil && json.Unmarshal(b, &s) == nil {
		return s.Time
	}
	return 0
}

// Reset forgets the base, so the next sync treats both sides as new (every
// host that differs becomes a conflict instead of being overwritten).
func Reset(dir string) error {
	err := os.Remove(statePath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

const maxKeySize = 64 << 10

// readKeys reads regular files directly in dir (no subdirectories).
func readKeys(dir string) (map[string]string, error) {
	out := map[string]string{}
	ents, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range ents {
		if !e.Type().IsRegular() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		if len(b) > maxKeySize {
			continue
		}
		out[e.Name()] = base64.StdEncoding.EncodeToString(b)
	}
	return out, nil
}

func validKeyName(n string) bool {
	return n != "" && n == filepath.Base(n) && !strings.HasPrefix(n, ".") && !strings.ContainsAny(n, `/\:`)
}

// writeKeys makes dir match want. Removed keys are moved to backupDir, never deleted.
func writeKeys(dir, backupDir string, have, want map[string]string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	for n, v := range want {
		if !validKeyName(n) {
			return fmt.Errorf("远端里有无效的密钥文件名 %q", n)
		}
		if have[n] == v {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o600)
		if strings.HasSuffix(n, ".pub") {
			mode = 0o644
		}
		p := filepath.Join(dir, n)
		tmp := filepath.Join(dir, ".tsshm-"+n)
		if err := os.WriteFile(tmp, b, mode); err != nil {
			return err
		}
		if err := os.Rename(tmp, p); err != nil {
			os.Remove(tmp)
			return err
		}
	}
	for n := range have {
		if _, ok := want[n]; ok || !validKeyName(n) {
			continue
		}
		if err := os.MkdirAll(backupDir, 0o700); err != nil {
			return err
		}
		dst := filepath.Join(backupDir, n+"."+time.Now().Format("20060102-150405"))
		if err := os.Rename(filepath.Join(dir, n), dst); err != nil {
			return err
		}
	}
	return nil
}

func sameKeys(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// Run performs one sync. Conflicts are returned without changing anything;
// call again with Options.Resolve covering every Conflict.ID.
func Run(ctx context.Context, o Options) (Result, error) {
	c := LoadConfig(o.SettingsDir)
	if err := c.Validate(); err != nil {
		return Result{}, err
	}
	be, err := c.backend()
	if err != nil {
		return Result{}, err
	}
	for attempt := 0; ; attempt++ {
		r, err := runOnce(ctx, o, c, be)
		if errors.Is(err, errRemoteChanged) && attempt < 3 {
			continue // another device uploaded in between: merge again
		}
		if err == nil {
			r.Warnings = append(r.Warnings, be.Warnings()...)
		}
		return r, err
	}
}

func runOnce(ctx context.Context, o Options, c Config, be Backend) (Result, error) {
	res := Result{Remote: be.Describe(), Time: time.Now().UnixMilli()}
	store, err := sshconf.NewStore(o.ConfigPath, filepath.Join(o.SettingsDir, "backups"))
	if err != nil {
		return res, err
	}
	doc, err := store.Load()
	if err != nil {
		return res, err
	}
	local := sshconf.Normalize(doc.String())
	localKeys := map[string]string{}
	if c.SyncKeys {
		if localKeys, err = readKeys(expand(o.KeyDir)); err != nil {
			return res, err
		}
	}

	var base snapshot
	hasBase := false
	if b, err := os.ReadFile(statePath(o.SettingsDir)); err == nil && json.Unmarshal(b, &base) == nil {
		hasBase = true
	}
	if !hasBase || !base.SyncKeys {
		// keys were not part of the last sync: treat them as new on both sides
		base.Keys = map[string]string{}
	}

	blob, rev, err := be.Get(ctx)
	var remote payload
	switch {
	case errors.Is(err, errRemoteMissing):
		rev = ""
		if hasBase {
			res.Warnings = append(res.Warnings, "远端文件不存在（可能被删除了），已用本机配置重新上传。")
		}
		remote = payload{Config: local, Keys: localKeys}
		base = snapshot{Config: local, Keys: localKeys}
		hasBase = true
	case err != nil:
		return res, err
	default:
		plain, err := open(c.Passphrase, blob)
		if err != nil {
			return res, err
		}
		if err := json.Unmarshal(plain, &remote); err != nil {
			return res, fmt.Errorf("远端文件格式不对：%w", err)
		}
		if remote.Format > 1 {
			return res, errors.New("远端文件来自更新版本的 tssh-manager，请先升级本机")
		}
		remote.Config = sshconf.Normalize(remote.Config)
		res.RemoteDevice = remote.Device
	}
	if remote.Keys == nil {
		remote.Keys = map[string]string{}
	}
	if !hasBase {
		base = snapshot{Keys: map[string]string{}}
	}

	merged, conflicts, ch, err := mergeConfig(base.Config, local, remote.Config, o.Resolve)
	if err != nil {
		return res, fmt.Errorf("合并失败：%w", err)
	}
	mergedKeys := localKeys
	if c.SyncKeys {
		var kc []Conflict
		var kch Changes
		mergedKeys, kc, kch = mergeFiles(base.Keys, localKeys, remote.Keys, o.Resolve)
		conflicts = append(conflicts, kc...)
		ch.FromRemote = append(ch.FromRemote, kch.FromRemote...)
		ch.FromLocal = append(ch.FromLocal, kch.FromLocal...)
	} else {
		mergedKeys = remote.Keys // leave what other devices uploaded alone
	}
	res.Changes = ch
	if len(conflicts) > 0 {
		res.Status = "conflict"
		res.Conflicts = conflicts
		sort.Slice(res.Conflicts, func(i, j int) bool { return res.Conflicts[i].ID < res.Conflicts[j].ID })
		return res, nil
	}

	wroteLocal := false
	if merged != local {
		nd, err := sshconf.Parse(merged)
		if err != nil {
			return res, err
		}
		if res.Backup, err = store.Save(nd); err != nil {
			return res, fmt.Errorf("写入本机配置失败：%w", err)
		}
		wroteLocal = true
	}
	if c.SyncKeys && !sameKeys(mergedKeys, localKeys) {
		if err := writeKeys(expand(o.KeyDir), filepath.Join(o.SettingsDir, "backups", "keys"), localKeys, mergedKeys); err != nil {
			return res, fmt.Errorf("写入密钥失败：%w", err)
		}
		wroteLocal = true
	}
	wroteRemote := false
	if rev == "" || merged != remote.Config || !sameKeys(mergedKeys, remote.Keys) {
		p := payload{Format: 1, Device: c.Device, Updated: time.Now().Unix(), Config: merged}
		if len(mergedKeys) > 0 {
			p.Keys = mergedKeys
		}
		plain, _ := json.Marshal(p)
		blob, err := seal(c.Passphrase, plain)
		if err != nil {
			return res, err
		}
		if err := be.Put(ctx, blob, rev); err != nil {
			return res, err
		}
		wroteRemote = true
	}
	keys := mergedKeys
	if !c.SyncKeys {
		keys = nil
	}
	if err := writeJSON(statePath(o.SettingsDir), snapshot{Config: merged, Keys: keys, SyncKeys: c.SyncKeys,
		Time: time.Now().UnixMilli(), Device: remote.Device}); err != nil {
		return res, err
	}
	switch {
	case wroteLocal && wroteRemote:
		res.Status = "merged"
	case wroteLocal:
		res.Status = "pulled"
	case wroteRemote:
		res.Status = "pushed"
	default:
		res.Status = "uptodate"
	}
	return res, nil
}
