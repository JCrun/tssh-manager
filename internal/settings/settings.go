// Package settings stores tssh-manager preferences in
// $XDG_CONFIG_HOME/tssh-manager/settings.json.
package settings

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Settings struct {
	// Terminal is the command that opens a terminal running tssh. {host} is
	// replaced by the alias as a separate argument; it is never run by a shell.
	Terminal   string `json:"terminal"`
	ConfigPath string `json:"configPath"`
	KeyDir     string `json:"keyDir"`
	TSSH       string `json:"tssh"`
}

func Dir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "tssh-manager")
}

func path() string { return filepath.Join(Dir(), "settings.json") }

// DetectTerminal picks the first installed terminal.
func DetectTerminal() string {
	for _, t := range []struct{ bin, cmd string }{
		{"ghostty", "ghostty -e tssh {host}"},
		{"kitty", "kitty tssh {host}"},
		{"wezterm", "wezterm start -- tssh {host}"},
		{"alacritty", "alacritty -e tssh {host}"},
		{"foot", "foot tssh {host}"},
		{"konsole", "konsole -e tssh {host}"},
		{"gnome-terminal", "gnome-terminal -- tssh {host}"},
		{"ptyxis", "ptyxis -- tssh {host}"},
		{"xfce4-terminal", "xfce4-terminal -x tssh {host}"},
		{"xterm", "xterm -e tssh {host}"},
	} {
		if _, err := exec.LookPath(t.bin); err == nil {
			return t.cmd
		}
	}
	return "xterm -e tssh {host}"
}

func Defaults() Settings {
	home, _ := os.UserHomeDir()
	return Settings{
		Terminal:   DetectTerminal(),
		ConfigPath: filepath.Join(home, ".ssh", "config"),
		KeyDir:     filepath.Join(home, ".ssh", "tabby-keys"),
		TSSH:       "tssh",
	}
}

func Load() Settings {
	s := Defaults()
	b, err := os.ReadFile(path())
	if err == nil {
		_ = json.Unmarshal(b, &s)
	}
	d := Defaults()
	if strings.TrimSpace(s.Terminal) == "" {
		s.Terminal = d.Terminal
	}
	if s.ConfigPath == "" {
		s.ConfigPath = d.ConfigPath
	}
	if s.KeyDir == "" {
		s.KeyDir = d.KeyDir
	}
	if s.TSSH == "" {
		s.TSSH = d.TSSH
	}
	return s
}

func Save(s Settings) error {
	if !strings.Contains(s.Terminal, "{host}") {
		return errors.New("terminal command must contain {host}")
	}
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(path(), append(b, '\n'), 0o600)
}

// Expand turns "~/" into the home directory.
func Expand(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}
