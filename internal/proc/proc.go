// Package proc starts external programs (ssh, tssh, ssh-keygen, terminals) in
// a way that works on Linux, macOS and Windows.
package proc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Command is exec.CommandContext, but on Windows the child gets no console
// window, so background checks do not flash windows over the GUI.
func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	hide(cmd)
	return cmd
}

// StartDetached starts a program that outlives this one (a terminal window).
// On Windows a console program gets its own console window.
func StartDetached(argv []string) error {
	cmd := exec.Command(argv[0], argv[1:]...)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// FixPath adds the usual install locations to PATH. GUI apps on macOS start
// with only /usr/bin:/bin:/usr/sbin:/sbin, so Homebrew's tssh would not be found.
func FixPath() {
	home, _ := os.UserHomeDir()
	var extra []string
	switch runtime.GOOS {
	case "darwin":
		extra = []string{"/opt/homebrew/bin", "/usr/local/bin", filepath.Join(home, "go", "bin"), filepath.Join(home, ".local", "bin")}
	case "linux":
		extra = []string{filepath.Join(home, ".local", "bin"), filepath.Join(home, "go", "bin")}
	case "windows":
		extra = []string{filepath.Join(home, "go", "bin"), filepath.Join(os.Getenv("ProgramFiles"), "OpenSSH")}
	}
	cur := filepath.SplitList(os.Getenv("PATH"))
	have := map[string]bool{}
	for _, p := range cur {
		have[p] = true
	}
	for _, p := range extra {
		if !have[p] {
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				cur = append(cur, p)
			}
		}
	}
	os.Setenv("PATH", strings.Join(cur, string(os.PathListSeparator)))
}

// Resolve returns the absolute path of bin if it is on PATH, else bin unchanged.
func Resolve(bin string) string {
	if p, err := exec.LookPath(bin); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}
	return bin
}
