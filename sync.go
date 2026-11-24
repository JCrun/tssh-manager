package main

import (
	"context"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tssh-manager/internal/proc"
	"tssh-manager/internal/settings"
	"tssh-manager/internal/syncer"
)

// Multi-device sync. The engine lives in internal/syncer and is shared with
// the tssh-sync command line tool.

// SyncView is the sync configuration without secrets.
type SyncView struct {
	Backend           string `json:"backend"`
	Folder            string `json:"folder"`
	WebDAVURL         string `json:"webdavURL"`
	WebDAVUser        string `json:"webdavUser"`
	HasWebDAVPassword bool   `json:"hasWebdavPassword"`
	HasPassphrase     bool   `json:"hasPassphrase"`
	SyncKeys          bool   `json:"syncKeys"`
	Auto              bool   `json:"auto"`
	Device            string `json:"device"`
	KeyDir            string `json:"keyDir"`
	LastSync          int64  `json:"lastSync"`
}

// SyncSave updates the configuration. Empty WebDAVPassword/Passphrase keep
// the stored values.
type SyncSave struct {
	Backend        string `json:"backend"`
	Folder         string `json:"folder"`
	WebDAVURL      string `json:"webdavURL"`
	WebDAVUser     string `json:"webdavUser"`
	WebDAVPassword string `json:"webdavPassword"`
	Passphrase     string `json:"passphrase"`
	SyncKeys       bool   `json:"syncKeys"`
	Auto           bool   `json:"auto"`
	Device         string `json:"device"`
}

type syncState struct {
	mu    sync.Mutex // one sync at a time
	timer *time.Timer
	tmu   sync.Mutex
}

func (a *App) GetSync() SyncView {
	c := syncer.LoadConfig(settings.Dir())
	return SyncView{Backend: c.Backend, Folder: c.Folder, WebDAVURL: c.WebDAVURL, WebDAVUser: c.WebDAVUser,
		HasWebDAVPassword: c.WebDAVPassword != "", HasPassphrase: c.Passphrase != "", SyncKeys: c.SyncKeys,
		Auto: c.Auto, Device: c.Device, KeyDir: a.set.KeyDir, LastSync: syncer.LastSync(settings.Dir())}
}

func (a *App) SaveSync(s SyncSave) error {
	dir := settings.Dir()
	old := syncer.LoadConfig(dir)
	c := syncer.Config{Backend: s.Backend, Folder: s.Folder, WebDAVURL: s.WebDAVURL, WebDAVUser: s.WebDAVUser,
		WebDAVPassword: old.WebDAVPassword, Passphrase: old.Passphrase, SyncKeys: s.SyncKeys, Auto: s.Auto, Device: s.Device}
	if s.WebDAVPassword != "" {
		c.WebDAVPassword = s.WebDAVPassword
	}
	if s.Passphrase != "" {
		c.Passphrase = s.Passphrase
	}
	if err := syncer.SaveConfig(dir, c); err != nil {
		return err
	}
	// a different remote has a different history
	if c.Backend != old.Backend || c.Folder != old.Folder || c.WebDAVURL != old.WebDAVURL {
		return syncer.Reset(dir)
	}
	return nil
}

// SyncReset forgets the last synced state. The next sync then reports every
// host that differs as a conflict instead of choosing a side.
func (a *App) SyncReset() error { return syncer.Reset(settings.Dir()) }

// SyncNow runs one sync. resolve maps Conflict.ID to "local" or "remote".
func (a *App) SyncNow(resolve map[string]string) (syncer.Result, error) {
	a.sync.mu.Lock()
	defer a.sync.mu.Unlock()
	a.mu.Lock() // no edits while the local file is merged
	defer a.mu.Unlock()
	ctx, cancel := context.WithTimeout(a.ctx, 90*time.Second)
	defer cancel()
	r, err := syncer.Run(ctx, syncer.Options{SettingsDir: settings.Dir(), ConfigPath: settings.Expand(a.set.ConfigPath),
		KeyDir: settings.Expand(a.set.KeyDir), Resolve: resolve})
	return r, err
}

type syncEvent struct {
	Result *syncer.Result `json:"result"`
	Error  string         `json:"error"`
}

// autoSync runs in the background and reports through the "sync:result" event.
func (a *App) autoSync() {
	if c := syncer.LoadConfig(settings.Dir()); c.Backend == "" || !c.Auto || a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "sync:running")
	r, err := a.SyncNow(nil)
	ev := syncEvent{Result: &r}
	if err != nil {
		ev = syncEvent{Error: err.Error()}
	}
	runtime.EventsEmit(a.ctx, "sync:result", ev)
}

// scheduleSync debounces auto sync after local edits.
func (a *App) scheduleSync() {
	if c := syncer.LoadConfig(settings.Dir()); c.Backend == "" || !c.Auto {
		return
	}
	a.sync.tmu.Lock()
	defer a.sync.tmu.Unlock()
	if a.sync.timer != nil {
		a.sync.timer.Stop()
	}
	a.sync.timer = time.AfterFunc(2*time.Second, a.autoSync)
}

// TSSHInfo describes the tssh that will be used.
type TSSHInfo struct {
	proc.TSSH
	Version string `json:"version"`
	Error   string `json:"error"`
}

func (a *App) GetTSSH() TSSHInfo {
	t := proc.FindTSSH(a.set.TSSH)
	info := TSSHInfo{TSSH: t, Version: t.Version()}
	if err := t.Err(); err != nil {
		info.Error = err.Error()
	}
	return info
}

// TerminalPresets lists terminal commands for this OS.
func (a *App) TerminalPresets() []string { return settings.TerminalPresets() }
