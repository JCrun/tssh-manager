package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tssh-manager/internal/sshconf"
)

// Plaintext "#!! Password" management. Passwords are only sent to the UI on an
// explicit reveal; copy goes straight to the clipboard from Go.

const clipboardClearAfter = 30 * time.Second

var (
	clipMu    sync.Mutex
	clipToken int
)

func (a *App) readPassword(alias string) (string, error) {
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
	if _, ok := d.LocalValue(alias, "Password", true); !ok {
		return "", errors.New("这台主机没有保存明文密码")
	}
	return d.Lookup(alias, "Password")
}

// RevealPassword returns the plaintext password stored for alias.
func (a *App) RevealPassword(alias string) (string, error) { return a.readPassword(alias) }

// CopyPassword puts the password on the clipboard and clears it after 30s,
// unless something else has been copied in the meantime.
func (a *App) CopyPassword(alias string) (int, error) {
	pw, err := a.readPassword(alias)
	if err != nil {
		return 0, err
	}
	if err := runtime.ClipboardSetText(a.ctx, pw); err != nil {
		return 0, err
	}
	clipMu.Lock()
	clipToken++
	tok := clipToken
	clipMu.Unlock()
	go func() {
		time.Sleep(clipboardClearAfter)
		clipMu.Lock()
		defer clipMu.Unlock()
		if tok != clipToken {
			return
		}
		if cur, err := runtime.ClipboardGetText(a.ctx); err == nil && cur == pw {
			runtime.ClipboardSetText(a.ctx, "")
		}
	}()
	return int(clipboardClearAfter.Seconds()), nil
}

// passwordForms are the ways a value can be written, tried in order until tssh
// would read it back unchanged.
func passwordForms(pw string) []string {
	esc := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(pw)
	return []string{pw, `"` + esc + `"`, `'` + pw + `'`}
}

// SetPassword writes "#!! Password" for alias. An empty value removes it.
// The result is read back with tssh's parser and must match exactly.
func (a *App) SetPassword(alias, pw string) (string, error) {
	if pw == "" {
		return a.mutate(func(d *sshconf.Doc) error {
			return d.Apply(alias, []sshconf.Change{{Key: "Password", Ext: true}})
		})
	}
	if strings.ContainsAny(pw, "\r\n\x00") {
		return "", errors.New("密码不能包含换行或空字符")
	}
	if strings.Contains(pw, "#") {
		return "", errors.New("密码含有 #：tssh 读取配置时会把 # 之后当作注释，这样的密码无法以明文保存在 ssh 配置里")
	}
	return a.mutate(func(d *sshconf.Doc) error {
		orig := d.String()
		for _, form := range passwordForms(pw) {
			if err := d.Apply(alias, []sshconf.Change{{Key: "Password", Value: form, Ext: true}}); err != nil {
				return err
			}
			if got, err := d.Lookup(alias, "Password"); err == nil && got == pw {
				return nil
			}
		}
		nd, _ := sshconf.Parse(orig)
		*d = *nd
		return fmt.Errorf("这个密码无法在 ssh 配置里原样保存（tssh 读回的值不一致），请换一个密码")
	})
}
