// Package settings stores tssh-manager preferences in
// <config dir>/tssh-manager/settings.json: $XDG_CONFIG_HOME or ~/.config on
// Linux, ~/Library/Application Support on macOS, %AppData% on Windows.
package settings

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	if base == "" && runtime.GOOS != "linux" {
		base, _ = os.UserConfigDir()
	}
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "tssh-manager")
}

func path() string { return filepath.Join(Dir(), "settings.json") }

type terminal struct{ bin, cmd string }

// MacTerminalApp is a pseudo command understood by the app: it opens
// Terminal.app through osascript, which cannot be expressed as plain argv.
const MacTerminalApp = "@terminal-app"

func terminals() []terminal {
	switch runtime.GOOS {
	case "darwin":
		return []terminal{
			{"/Applications/Ghostty.app", "open -na Ghostty --args -e tssh {host}"},
			{"/Applications/WezTerm.app", "open -na WezTerm --args start -- tssh {host}"},
			{"/Applications/kitty.app", "open -na kitty --args tssh {host}"},
			{"/Applications/Alacritty.app", "open -na Alacritty --args -e tssh {host}"},
			{"/System/Applications/Utilities/Terminal.app", MacTerminalApp + " tssh {host}"},
		}
	case "windows":
		return []terminal{
			{"wt.exe", "wt.exe tssh {host}"},
			{"wezterm.exe", "wezterm.exe start -- tssh {host}"},
			{"", "tssh {host}"}, // its own console window
		}
	}
	return []terminal{
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
	}
}

func installed(bin string) bool {
	if bin == "" {
		return true
	}
	if filepath.IsAbs(bin) {
		_, err := os.Stat(bin)
		return err == nil
	}
	_, err := exec.LookPath(bin)
	return err == nil
}

// DetectTerminal picks the first installed terminal.
func DetectTerminal() string {
	ts := terminals()
	for _, t := range ts {
		if installed(t.bin) {
			return t.cmd
		}
	}
	return ts[len(ts)-1].cmd
}

// TerminalPresets lists the terminal commands for this OS, installed ones first.
func TerminalPresets() []string {
	var a, b []string
	for _, t := range terminals() {
		if installed(t.bin) {
			a = append(a, t.cmd)
		} else {
			b = append(b, t.cmd)
		}
	}
	return append(a, b...)
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
	if strings.HasPrefix(p, "~/") || (runtime.GOOS == "windows" && strings.HasPrefix(p, `~\`)) {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}
