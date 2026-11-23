package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tssh-manager/internal/checker"
	"tssh-manager/internal/settings"
	"tssh-manager/internal/sshconf"
)

// App is bound to the frontend. Every mutating call re-reads the config from
// disk, applies one change and saves, so the file stays the source of truth.
type App struct {
	ctx context.Context
	mu  sync.Mutex
	set settings.Settings
}

func NewApp() *App { return &App{set: settings.Load()} }

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

const mask = "******"

var (
	sensitiveRe = regexp.MustCompile(`(?i)(password|passphrase|secret|otp)`)
	includeRe   = regexp.MustCompile(`(?i)^\s*include\b`)
	keyNameRe   = regexp.MustCompile(`[^A-Za-z0-9_-]+`)
)

// HostView is one Host block with secrets removed.
type HostView struct {
	Alias          string            `json:"alias"`
	Patterns       []string          `json:"patterns"`
	Wildcard       bool              `json:"wildcard"`
	Section        string            `json:"section"`
	Line           int               `json:"line"`
	HostName       string            `json:"hostName"`
	Port           string            `json:"port"`
	User           string            `json:"user"`
	IdentityFile   string            `json:"identityFile"`
	IdentitiesOnly bool              `json:"identitiesOnly"`
	ProxyJump      string            `json:"proxyJump"`
	Groups         []string          `json:"groups"`
	Auth           string            `json:"auth"` // key | password | key+password | default
	PlainPassword  bool              `json:"plainPassword"`
	EncPassword    bool              `json:"encPassword"`
	Effective      map[string]string `json:"effective"`
	Options        []sshconf.Option  `json:"options"`
	Raw            string            `json:"raw"`
}

type State struct {
	ConfigPath string     `json:"configPath"`
	Hosts      []HostView `json:"hosts"`
	Sections   []string   `json:"sections"`
	Warnings   []string   `json:"warnings"`
	LoadedAt   int64      `json:"loadedAt"`
}

func (a *App) store() (*sshconf.Store, error) {
	return sshconf.NewStore(settings.Expand(a.set.ConfigPath), filepath.Join(settings.Dir(), "backups"))
}

func cutKV(s string) (string, string, bool) {
	i := strings.IndexAny(s, " \t=")
	if i < 0 {
		return s, "", false
	}
	v := strings.TrimPrefix(strings.TrimLeft(s[i:], " \t"), "=")
	return s[:i], strings.TrimSpace(v), true
}

// redactLine masks the value of password/secret options in one config line.
func redactLine(l string) string {
	t := strings.TrimSpace(l)
	isExt := strings.HasPrefix(t, "#!!")
	if strings.HasPrefix(t, "#") && !isExt {
		return l
	}
	body := t
	if isExt {
		body = strings.TrimSpace(t[3:])
	}
	k, v, ok := cutKV(body)
	if !ok || v == "" || !sensitiveRe.MatchString(k) {
		return l
	}
	return l[:strings.Index(l, k)+len(k)] + " " + mask
}

func view(e sshconf.Entry) HostView {
	h := HostView{Alias: e.Alias, Patterns: e.Patterns, Wildcard: e.Wildcard, Section: e.Section,
		Line: e.Line, Effective: e.Effective}
	var lines []string
	for _, l := range strings.Split(e.Raw, "\n") {
		lines = append(lines, redactLine(l))
	}
	h.Raw = strings.Join(lines, "\n")
	for _, o := range e.Options {
		k := strings.ToLower(o.Key)
		if o.Ext {
			switch k {
			case "password":
				h.PlainPassword = true
			case "encpassword":
				h.EncPassword = true
			case "grouplabels":
				h.Groups = strings.Fields(o.Value)
			}
		} else {
			switch k {
			case "hostname":
				h.HostName = o.Value
			case "port":
				h.Port = o.Value
			case "user":
				h.User = o.Value
			case "identityfile":
				if h.IdentityFile == "" {
					h.IdentityFile = o.Value
				}
			case "identitiesonly":
				h.IdentitiesOnly = strings.EqualFold(o.Value, "yes")
			case "proxyjump":
				h.ProxyJump = o.Value
			}
		}
		if sensitiveRe.MatchString(o.Key) && o.Value != "" {
			o.Value = mask
		}
		h.Options = append(h.Options, o)
	}
	pw := h.PlainPassword || h.EncPassword
	switch key := h.IdentityFile != ""; {
	case key && pw:
		h.Auth = "key+password"
	case key:
		h.Auth = "key"
	case pw:
		h.Auth = "password"
	default:
		h.Auth = "default"
	}
	return h
}

// Load returns the current config.
func (a *App) Load() (State, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, err := a.store()
	if err != nil {
		return State{}, err
	}
	d, err := s.Load()
	if err != nil {
		return State{}, err
	}
	st := State{ConfigPath: s.Path, Sections: d.Sections(), LoadedAt: time.Now().UnixMilli()}
	seen := map[string]int{}
	for _, e := range d.Entries() {
		st.Hosts = append(st.Hosts, view(e))
		if !e.Wildcard {
			seen[e.Alias]++
		}
	}
	for alias, n := range seen {
		if n > 1 {
			st.Warnings = append(st.Warnings, fmt.Sprintf("Host %s 定义了 %d 次，只能在文件里手动修改。", alias, n))
		}
	}
	for _, l := range strings.Split(d.String(), "\n") {
		if includeRe.MatchString(l) {
			st.Warnings = append(st.Warnings, "配置使用了 Include，被包含文件里的主机不会显示。")
			break
		}
	}
	return st, nil
}

// mutate loads, applies fn, saves and returns the backup path.
func (a *App) mutate(fn func(d *sshconf.Doc) error) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, err := a.store()
	if err != nil {
		return "", err
	}
	d, err := s.Load()
	if err != nil {
		return "", err
	}
	if err := fn(d); err != nil {
		return "", err
	}
	return s.Save(d)
}

// SaveRequest creates (OrigAlias == "") or updates a host from the form.
type SaveRequest struct {
	OrigAlias string           `json:"origAlias"`
	Alias     string           `json:"alias"`
	Section   string           `json:"section"`
	Changes   []sshconf.Change `json:"changes"`
}

func (a *App) SaveHost(r SaveRequest) (string, error) {
	for _, c := range r.Changes {
		if sensitiveRe.MatchString(c.Key) {
			return "", fmt.Errorf("%s 不能通过表单设置", c.Key)
		}
	}
	return a.mutate(func(d *sshconf.Doc) error {
		if r.OrigAlias == "" {
			return d.Add(r.Alias, r.Section, r.Changes)
		}
		if r.Alias != r.OrigAlias {
			if err := d.Rename(r.OrigAlias, r.Alias); err != nil {
				return err
			}
		}
		return d.Apply(r.Alias, r.Changes)
	})
}

// SaveRaw replaces a block with edited raw text. Masked secrets are restored
// from the current file.
func (a *App) SaveRaw(alias, raw string) (string, error) {
	return a.mutate(func(d *sshconf.Doc) error {
		var orig []sshconf.Option
		for _, e := range d.Entries() {
			if !e.Wildcard && e.Alias == alias && len(e.Patterns) == 1 {
				orig = e.Options
			}
		}
		lines := strings.Split(raw, "\n")
		for i, l := range lines {
			t := strings.TrimSpace(l)
			ext := strings.HasPrefix(t, "#!!")
			body := t
			if ext {
				body = strings.TrimSpace(t[3:])
			}
			k, v, _ := cutKV(body)
			if v != mask {
				continue
			}
			found := false
			for _, o := range orig {
				if o.Ext == ext && strings.EqualFold(o.Key, k) {
					lines[i] = strings.Replace(l, mask, o.Value, 1)
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("第 %d 行：%s 是遮罩值，但原配置里没有可保留的值", i+1, k)
			}
		}
		return d.ReplaceRaw(alias, strings.Join(lines, "\n"))
	})
}

func (a *App) DeleteHost(alias string) (string, error) {
	return a.mutate(func(d *sshconf.Doc) error { return d.Delete(alias) })
}

// RemovePassword deletes a stored "#!! Password" / "#!! encPassword".
func (a *App) RemovePassword(alias string) (string, error) {
	return a.mutate(func(d *sshconf.Doc) error {
		return d.Apply(alias, []sshconf.Change{{Key: "Password", Ext: true}, {Key: "encPassword", Ext: true}})
	})
}

// ---- terminal ----

func (a *App) terminalArgv(alias string, command []string) ([]string, error) {
	parts := strings.Fields(a.set.Terminal)
	if len(parts) == 0 {
		return nil, errors.New("终端命令为空")
	}
	if command == nil {
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			switch p {
			case "{host}":
				out = append(out, alias)
			case "tssh":
				out = append(out, a.set.TSSH)
			default:
				out = append(out, p)
			}
		}
		return out, nil
	}
	// run another command: keep everything before the "tssh" word
	for i, p := range parts {
		if p == "tssh" {
			return append(append([]string{}, parts[:i]...), command...), nil
		}
	}
	return nil, errors.New(`终端命令里需要有单独的 "tssh"，才能用它运行其他命令`)
}

func startDetached(argv []string) error {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// Connect opens the configured terminal running "tssh alias".
func (a *App) Connect(alias string) error {
	if !sshconf.ValidAlias(alias) {
		return fmt.Errorf("invalid alias %q", alias)
	}
	argv, err := a.terminalArgv(alias, nil)
	if err != nil {
		return err
	}
	return startDetached(argv)
}

// ---- checks ----

func (a *App) checkOpts() checker.Options {
	return checker.Options{SSH: "ssh", TSSH: a.set.TSSH, ConfigPath: settings.Expand(a.set.ConfigPath),
		TCPTimeout: 5 * time.Second, AuthTimeout: 25 * time.Second}
}

// Check tests hosts in parallel and emits "check:start"/"check:result" per host, then "check:done".
func (a *App) Check(aliases []string) {
	o := a.checkOpts()
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, al := range aliases {
		if !sshconf.ValidAlias(al) {
			continue
		}
		wg.Add(1)
		go func(al string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			runtime.EventsEmit(a.ctx, "check:start", al)
			runtime.EventsEmit(a.ctx, "check:result", checker.Check(a.ctx, o, al))
		}(al)
	}
	go func() { wg.Wait(); runtime.EventsEmit(a.ctx, "check:done") }()
}

// ---- keys ----

type KeyInfo struct {
	Path    string `json:"path"`
	Display string `json:"display"` // ~/ form, as written to the config
	Public  string `json:"public"`
	Created bool   `json:"created"`
}

func tilde(p string) string {
	home, _ := os.UserHomeDir()
	if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") {
		return "~/" + rel
	}
	return p
}

// GenerateKey creates (or reuses) <keyDir>/<alias>_ed25519.
func (a *App) GenerateKey(alias string) (KeyInfo, error) {
	if !sshconf.ValidAlias(alias) {
		return KeyInfo{}, fmt.Errorf("invalid alias %q", alias)
	}
	dir := settings.Expand(a.set.KeyDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return KeyInfo{}, err
	}
	p := filepath.Join(dir, strings.ToLower(keyNameRe.ReplaceAllString(alias, "-"))+"_ed25519")
	ki := KeyInfo{Path: p, Display: tilde(p)}
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "tssh-manager:"+alias, "-f", p).CombinedOutput()
		if err != nil {
			return KeyInfo{}, fmt.Errorf("ssh-keygen: %s", strings.TrimSpace(string(out)))
		}
		ki.Created = true
	}
	pub, err := os.ReadFile(p + ".pub")
	if err != nil {
		return KeyInfo{}, err
	}
	ki.Public = strings.TrimSpace(string(pub))
	return ki, nil
}

const installScript = `umask 077; mkdir -p ~/.ssh && k=$(cat) && touch ~/.ssh/authorized_keys && (grep -qxF "$k" ~/.ssh/authorized_keys || printf '%s\n' "$k" >> ~/.ssh/authorized_keys) && echo TSSHM_KEY_OK`

// InstallKey appends the public key on the server. Hosts with a stored tssh
// password are done in the background; others open a terminal with ssh-copy-id.
// Returns "done" or "terminal".
func (a *App) InstallKey(alias, keyPath string, hasStoredPassword bool) (string, error) {
	if !sshconf.ValidAlias(alias) {
		return "", fmt.Errorf("invalid alias %q", alias)
	}
	pub, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		return "", err
	}
	if !hasStoredPassword {
		argv, err := a.terminalArgv(alias, []string{"ssh-copy-id", "-i", keyPath + ".pub", alias})
		if err != nil {
			return "", err
		}
		return "terminal", startDetached(argv)
	}
	ctx, cancel := context.WithTimeout(a.ctx, 40*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.set.TSSH, "-T", "-F", settings.Expand(a.set.ConfigPath), alias, installScript)
	cmd.Stdin = strings.NewReader(strings.TrimSpace(string(pub)) + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "TSSHM_KEY_OK") {
		msg := strings.TrimSpace(string(out))
		if i := strings.LastIndex(msg, "\n"); i >= 0 {
			msg = msg[i+1:]
		}
		return "", fmt.Errorf("安装公钥失败：%s", msg)
	}
	return "done", nil
}

// VerifyKey logs in with only this key: no passwords, no other keys, no agent.
func (a *App) VerifyKey(alias, keyPath string) error {
	r, err := checker.Resolve(a.ctx, "ssh", settings.Expand(a.set.ConfigPath), alias)
	if err != nil {
		return err
	}
	if r.ProxyJump != "" || r.ProxyCommand != "" {
		return errors.New("经 ProxyJump/ProxyCommand 的主机暂不支持自动验证")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ssh", "-F", "/dev/null", "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes",
		"-o", "IdentityAgent=none", "-o", "PasswordAuthentication=no", "-o", "KbdInteractiveAuthentication=no",
		"-o", "ConnectTimeout=8", "-o", "StrictHostKeyChecking=accept-new",
		"-i", keyPath, "-p", r.Port, "-l", r.User, r.HostName, "true")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("密钥登录失败：%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// SwitchToKey verifies the key, then sets IdentityFile/IdentitiesOnly and
// removes stored passwords.
func (a *App) SwitchToKey(alias, keyPath string) (string, error) {
	if err := a.VerifyKey(alias, keyPath); err != nil {
		return "", err
	}
	return a.mutate(func(d *sshconf.Doc) error {
		return d.Apply(alias, []sshconf.Change{
			{Key: "IdentityFile", Value: tilde(keyPath)},
			{Key: "IdentitiesOnly", Value: "yes"},
			{Key: "Password", Ext: true},
			{Key: "encPassword", Ext: true},
		})
	})
}

// ---- settings & backups ----

func (a *App) GetSettings() settings.Settings { return a.set }

// Version returns the build version ("dev" for local builds).
func (a *App) Version() string { return version }

func (a *App) SaveSettings(s settings.Settings) error {
	if err := settings.Save(s); err != nil {
		return err
	}
	a.set = settings.Load()
	return nil
}

type Backup struct {
	Path string `json:"path"`
	Time int64  `json:"time"`
	Size int64  `json:"size"`
}

func (a *App) ListBackups() ([]Backup, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	var out []Backup
	for _, p := range s.Backups() {
		if st, err := os.Stat(p); err == nil {
			out = append(out, Backup{Path: p, Time: st.ModTime().UnixMilli(), Size: st.Size()})
		}
	}
	return out, nil
}

// RestoreBackup writes a backup back as the config (the current file is backed up first).
func (a *App) RestoreBackup(p string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, err := a.store()
	if err != nil {
		return "", err
	}
	if filepath.Dir(filepath.Clean(p)) != filepath.Clean(s.BackupDir) {
		return "", errors.New("不是 tssh-manager 的备份文件")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	nd, err := sshconf.Parse(string(b))
	if err != nil {
		return "", err
	}
	if _, err := s.Load(); err != nil {
		return "", err
	}
	return s.Save(nd)
}
