package proc

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// TSSH says which tssh binary will be used.
type TSSH struct {
	Path   string `json:"path"`
	Source string `json:"source"` // setting | path | bundled | missing
}

func exeName(n string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(n), ".exe") {
		return n + ".exe"
	}
	return n
}

// bundledTSSH is the official tssh shipped next to the app: beside
// tssh-manager(.exe), or in tssh-manager.app/Contents/MacOS.
func bundledTSSH() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	p := filepath.Join(filepath.Dir(exe), exeName("tssh"))
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p
	}
	return ""
}

// FindTSSH resolves the tssh to run. A custom setting (anything but "tssh")
// is used as given. Otherwise a tssh on PATH wins, so a user-installed
// version is preferred, and the bundled copy is the fallback.
func FindTSSH(setting string) TSSH {
	s := strings.TrimSpace(setting)
	if s != "" && s != "tssh" {
		if p, err := exec.LookPath(s); err == nil {
			return TSSH{Path: absPath(p), Source: "setting"}
		}
		return TSSH{Path: s, Source: "missing"}
	}
	if p, err := exec.LookPath("tssh"); err == nil {
		return TSSH{Path: absPath(p), Source: "path"}
	}
	if p := bundledTSSH(); p != "" {
		return TSSH{Path: p, Source: "bundled"}
	}
	return TSSH{Path: "tssh", Source: "missing"}
}

func absPath(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

// Version runs "tssh -V".
func (t TSSH) Version() string {
	if t.Source == "missing" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := Command(ctx, t.Path, "-V").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// TSSHInstallHint is the official way to install tssh on this OS
// (https://github.com/trzsz/trzsz-ssh).
func TSSHInstallHint() string {
	switch runtime.GOOS {
	case "windows":
		return "安装方法：winget install tssh（或 scoop install tssh / choco install tssh），装好后重启 tssh-manager。"
	case "darwin":
		return "安装方法：brew install trzsz-ssh，装好后重启 tssh-manager。"
	}
	return "安装方法：Arch 用 yay -S tssh；Ubuntu/Debian 先 sudo add-apt-repository ppa:trzsz/ppa 再 sudo apt install tssh；" +
		"或 go install github.com/trzsz/trzsz-ssh/cmd/tssh@latest。官方说明：https://github.com/trzsz/trzsz-ssh"
}

// Err explains a missing tssh, or returns nil.
func (t TSSH) Err() error {
	if t.Source != "missing" {
		return nil
	}
	if t.Path != "tssh" {
		return errors.New("设置里的 tssh 程序找不到：" + t.Path + "。把设置改回 tssh 可使用自带或已安装的版本。")
	}
	return errors.New("没有找到 tssh。" + TSSHInstallHint())
}
