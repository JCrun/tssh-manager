// Package checker tests reachability and login of ssh config hosts using the
// same client the user connects with (tssh), never interactively.
package checker

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"strconv"
	"strings"
	"time"

	"tssh-manager/internal/proc"
)

// Result of checking one host.
type Result struct {
	Alias    string `json:"alias"`
	Target   string `json:"target"`   // host:port actually dialled, from ssh -G
	TCP      string `json:"tcp"`      // ok | fail | skipped (proxy)
	TCPMs    int64  `json:"tcpMs"`    // connect time
	Login    string `json:"login"`    // ok | fail | skipped
	LoginMs  int64  `json:"loginMs"`  // full tssh login + "true"
	Problem  string `json:"problem"`  // short classification: dns, refused, timeout, auth, hostkey, ...
	Message  string `json:"message"`  // last line of client output
	Finished int64  `json:"finished"` // unix ms
}

// Resolved holds the effective settings OpenSSH computes for an alias.
type Resolved struct {
	HostName, Port, User, ProxyJump, ProxyCommand string
}

// Resolve runs "ssh -G alias", which applies Host * defaults, Match blocks and Includes.
func Resolve(ctx context.Context, sshBin, configPath, alias string) (Resolved, error) {
	args := []string{"-G"}
	if configPath != "" {
		args = append(args, "-F", configPath)
	}
	out, err := proc.Command(ctx, sshBin, append(args, alias)...).Output()
	if err != nil {
		return Resolved{}, err
	}
	var r Resolved
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		k, v, _ := strings.Cut(sc.Text(), " ")
		switch k {
		case "hostname":
			r.HostName = v
		case "port":
			r.Port = v
		case "user":
			r.User = v
		case "proxyjump":
			r.ProxyJump = v
		case "proxycommand":
			r.ProxyCommand = v
		}
	}
	return r, nil
}

// Options for Check.
type Options struct {
	SSH, TSSH   string // binaries
	ConfigPath  string // passed with -F when set
	TCPTimeout  time.Duration
	AuthTimeout time.Duration
	SkipLogin   bool
}

// Check dials the host and then runs "tssh -T alias true" with no stdin.
func Check(ctx context.Context, o Options, alias string) Result {
	res := Result{Alias: alias, TCP: "skipped", Login: "skipped"}
	defer func() { res.Finished = time.Now().UnixMilli() }()
	r, err := Resolve(ctx, o.SSH, o.ConfigPath, alias)
	if err != nil {
		res.Problem, res.Message = "config", "ssh -G failed: "+err.Error()
		return res
	}
	res.Target = net.JoinHostPort(r.HostName, r.Port)
	proxied := r.ProxyJump != "" || r.ProxyCommand != ""
	if !proxied {
		t := time.Now()
		var d net.Dialer
		dctx, cancel := context.WithTimeout(ctx, o.TCPTimeout)
		c, err := d.DialContext(dctx, "tcp", res.Target)
		cancel()
		res.TCPMs = time.Since(t).Milliseconds()
		if err != nil {
			res.TCP = "fail"
			res.Problem, res.Message = classify(err.Error()), err.Error()
			return res
		}
		c.Close()
		res.TCP = "ok"
	}
	if o.SkipLogin {
		return res
	}
	args := []string{"-T"}
	if o.ConfigPath != "" {
		args = append(args, "-F", o.ConfigPath)
	}
	args = append(args, "-o", "ConnectTimeout="+strconv.Itoa(int(o.TCPTimeout.Seconds())), alias, "true")
	actx, cancel := context.WithTimeout(ctx, o.AuthTimeout)
	defer cancel()
	cmd := proc.Command(actx, o.TSSH, args...)
	cmd.Stdin = nil // /dev/null: any prompt fails instead of hanging
	t := time.Now()
	out, err := cmd.CombinedOutput()
	res.LoginMs = time.Since(t).Milliseconds()
	if err == nil {
		res.Login = "ok"
		return res
	}
	res.Login = "fail"
	msg := lastLine(string(out))
	if actx.Err() == context.DeadlineExceeded {
		msg = "login did not finish within " + o.AuthTimeout.String() + " (waiting for a prompt?)"
		res.Problem = "timeout"
	} else {
		res.Problem = classify(msg)
	}
	res.Message = msg
	return res
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l != "" && !strings.HasPrefix(l, "Pseudo-terminal will not be allocated") {
			if len(l) > 300 {
				l = l[:300] + "…"
			}
			return l
		}
	}
	return "exit status non-zero"
}

func classify(s string) string {
	l := strings.ToLower(s)
	switch {
	case strings.Contains(l, "no such host"), strings.Contains(l, "could not resolve"):
		return "dns"
	case strings.Contains(l, "refused"):
		return "refused"
	case strings.Contains(l, "timeout"), strings.Contains(l, "timed out"):
		return "timeout"
	case strings.Contains(l, "no route"), strings.Contains(l, "unreachable"):
		return "unreachable"
	case strings.Contains(l, "host key"), strings.Contains(l, "known_hosts"), strings.Contains(l, "fingerprint"):
		return "hostkey"
	case strings.Contains(l, "auth"), strings.Contains(l, "permission denied"), strings.Contains(l, "password"):
		return "auth"
	case strings.Contains(l, "reset"), strings.Contains(l, "eof"), strings.Contains(l, "closed"):
		return "closed"
	}
	return "other"
}
