// tssh-sync is the command line side of tssh-manager's sync. It uses the same
// settings, sync configuration and encrypted remote file as the GUI, so it can
// run on headless servers, in Termux, or from cron.
//
//	tssh-sync                      sync once (same as "tssh-sync run")
//	tssh-sync run [--prefer local|remote]
//	tssh-sync status
//	tssh-sync setup folder <dir> [--keys] [--device NAME]
//	tssh-sync setup webdav <url> [--user USER] [--keys] [--device NAME]
//	tssh-sync reset                forget the last synced state
//	tssh-sync off                  turn sync off (keeps the remote file)
//
// The passphrase and WebDAV password are read from the terminal, or from
// TSSHM_SYNC_PASSPHRASE and TSSHM_WEBDAV_PASSWORD.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"

	"tssh-manager/internal/proc"
	"tssh-manager/internal/settings"
	"tssh-manager/internal/syncer"
)

var version = "dev"

func main() {
	proc.FixPath()
	args := os.Args[1:]
	cmd := "run"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "run":
		err = run(args)
	case "status":
		err = status()
	case "setup":
		err = setup(args)
	case "reset":
		err = syncer.Reset(settings.Dir())
		if err == nil {
			fmt.Println("已忘记上次同步的状态；下次同步时两边不同的主机都会作为冲突列出。")
		}
	case "off":
		c := syncer.LoadConfig(settings.Dir())
		c.Backend = ""
		err = syncer.SaveConfig(settings.Dir(), c)
	case "version", "--version":
		fmt.Println("tssh-sync", version)
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "tssh-sync:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage:
  tssh-sync [run] [--prefer local|remote]
  tssh-sync status
  tssh-sync setup folder <dir> [--keys] [--device NAME]
  tssh-sync setup webdav <url> [--user USER] [--keys] [--device NAME]
  tssh-sync reset | off | version
settings: `+settings.Dir()+"\n")
}

func run(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	prefer := fs.String("prefer", "", "resolve every conflict with this side: local or remote")
	fs.Parse(args)
	if *prefer != "" && *prefer != "local" && *prefer != "remote" {
		return errors.New("--prefer must be local or remote")
	}
	set := settings.Load()
	o := syncer.Options{SettingsDir: settings.Dir(), ConfigPath: settings.Expand(set.ConfigPath), KeyDir: settings.Expand(set.KeyDir)}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	r, err := syncer.Run(ctx, o)
	if err != nil {
		return err
	}
	if r.Status == "conflict" && *prefer != "" {
		o.Resolve = map[string]string{}
		for _, c := range r.Conflicts {
			o.Resolve[c.ID] = *prefer
		}
		if r, err = syncer.Run(ctx, o); err != nil {
			return err
		}
	}
	report(r)
	if r.Status == "conflict" {
		os.Exit(3)
	}
	return nil
}

func report(r syncer.Result) {
	msg := map[string]string{"uptodate": "已是最新", "pulled": "已从远端更新本机", "pushed": "已上传到远端",
		"merged": "已合并（本机和远端都更新了）", "conflict": "有冲突，没有做任何修改"}[r.Status]
	fmt.Printf("%s — %s\n", msg, r.Remote)
	for _, n := range r.Changes.FromRemote {
		fmt.Println("  ↓", n)
	}
	for _, n := range r.Changes.FromLocal {
		fmt.Println("  ↑", n)
	}
	if r.Backup != "" {
		fmt.Println("  本机原配置已备份到", r.Backup)
	}
	for _, w := range r.Warnings {
		fmt.Println("  注意：", w)
	}
	for _, c := range r.Conflicts {
		fmt.Printf("\n冲突 %s（%s）%s\n", c.Name, c.ID, c.Note)
		show := func(label, s string) {
			if s == "" {
				s = "（已删除）\n"
			}
			fmt.Printf("--- %s\n%s", label, s)
		}
		show("本机", c.Local)
		show("远端", c.Remote)
	}
	if r.Status == "conflict" {
		fmt.Println("\n用 tssh-sync run --prefer local 或 --prefer remote 解决，或在 tssh-manager 里逐个选择。")
	}
}

func status() error {
	c := syncer.LoadConfig(settings.Dir())
	if c.Backend == "" {
		fmt.Println("同步未开启。用 tssh-sync setup 开启。")
		return nil
	}
	where := c.Folder
	if c.Backend == "webdav" {
		where = c.WebDAVURL
	}
	fmt.Printf("方式：%s %s\n设备：%s\n同步密钥：%v\n自动同步（GUI）：%v\n", c.Backend, where, c.Device, c.SyncKeys, c.Auto)
	if t := syncer.LastSync(settings.Dir()); t > 0 {
		fmt.Println("上次同步：", time.UnixMilli(t).Format("2006-01-02 15:04:05"))
	} else {
		fmt.Println("上次同步：从未")
	}
	return nil
}

func secret(prompt, env string, confirm bool) (string, error) {
	if v := os.Getenv(env); v != "" {
		return v, nil
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		return strings.TrimRight(line, "\r\n"), err
	}
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil || !confirm {
		return string(b), err
	}
	fmt.Fprint(os.Stderr, "再输入一次：")
	b2, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(b) != string(b2) {
		return "", errors.New("两次输入不一致")
	}
	return string(b), nil
}

func setup(args []string) error {
	if len(args) < 2 {
		usage()
		os.Exit(2)
	}
	kind, where := args[0], args[1]
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	user := fs.String("user", "", "WebDAV user")
	keys := fs.Bool("keys", false, "also sync private keys in the key directory")
	device := fs.String("device", "", "name of this device")
	fs.Parse(args[2:])
	dir := settings.Dir()
	c := syncer.LoadConfig(dir)
	old := c
	c.Backend, c.SyncKeys = kind, *keys
	if *device != "" {
		c.Device = *device
	}
	switch kind {
	case "folder":
		abs, err := filepath.Abs(settings.Expand(where))
		if err != nil {
			return err
		}
		c.Folder = abs
	case "webdav":
		c.WebDAVURL, c.WebDAVUser = where, *user
		pw, err := secret("WebDAV 密码（坚果云等请用应用密码）：", "TSSHM_WEBDAV_PASSWORD", false)
		if err != nil {
			return err
		}
		c.WebDAVPassword = pw
	default:
		return fmt.Errorf("unknown backend %q (folder or webdav)", kind)
	}
	pp, err := secret("同步口令（所有设备相同，至少 8 个字符）：", "TSSHM_SYNC_PASSPHRASE", true)
	if err != nil {
		return err
	}
	c.Passphrase = pp
	if err := syncer.SaveConfig(dir, c); err != nil {
		return err
	}
	if c.Backend != old.Backend || c.Folder != old.Folder || c.WebDAVURL != old.WebDAVURL {
		if err := syncer.Reset(dir); err != nil {
			return err
		}
	}
	fmt.Println("已保存到", syncer.ConfigPath(dir), "，现在同步一次：")
	return run(nil)
}
