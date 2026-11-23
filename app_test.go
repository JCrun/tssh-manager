package main

import (
	"strings"
	"testing"

	"tssh-manager/internal/sshconf"
)

func TestViewRedactsSecrets(t *testing.T) {
	d, err := sshconf.Parse("Host a\n    HostName h\n    #!! Password hunter2\n    #!! encPassword abc123\n    #!! TotpSecret1 JBSWY3DP\n    #!! GroupLabels x y\n")
	if err != nil {
		t.Fatal(err)
	}
	v := view(d.Entries()[0])
	for _, bad := range []string{"hunter2", "abc123", "JBSWY3DP"} {
		if strings.Contains(v.Raw, bad) {
			t.Errorf("raw leaks %s:\n%s", bad, v.Raw)
		}
		for _, o := range v.Options {
			if o.Value == bad {
				t.Errorf("option %s leaks value", o.Key)
			}
		}
	}
	if !v.PlainPassword || !v.EncPassword || v.Auth != "password" || strings.Join(v.Groups, ",") != "x,y" {
		t.Errorf("view: %+v", v)
	}
	if !strings.Contains(v.Raw, "#!! Password ******") {
		t.Errorf("mask missing:\n%s", v.Raw)
	}
}

func TestRedactLineLeavesOthers(t *testing.T) {
	for _, l := range []string{"    HostName 1.2.3.4", "# Password in a comment stays", "    #!! GroupLabels a"} {
		if redactLine(l) != l {
			t.Errorf("changed %q -> %q", l, redactLine(l))
		}
	}
	if got := redactLine("\t#!!Password=secret"); got != "\t#!!Password ******" {
		t.Errorf("got %q", got)
	}
}

func TestSetPasswordForms(t *testing.T) {
	cases := map[string]bool{"simple": true, "with space": true, `q"uote`: true, "tab\there": true, " lead": true,
		`back\slash`: true, "日本語パス": true, "a#b": false, "a #b": false}
	for pw, want := range cases {
		d, _ := sshconf.Parse("Host a\n    HostName h\n")
		ok := false
		for _, f := range passwordForms(pw) {
			d.Apply("a", []sshconf.Change{{Key: "Password", Value: f, Ext: true}})
			if got, _ := d.Lookup("a", "Password"); got == pw {
				ok = true
				break
			}
		}
		if ok != want {
			t.Errorf("%q: storable=%v, want %v", pw, ok, want)
		}
	}
}

func TestExplainKeyFailure(t *testing.T) {
	if e := explainKeyFailure("root@1.2.3.4: Permission denied (password)."); !strings.Contains(e.Error(), "PubkeyAuthentication no") {
		t.Errorf("password-only not explained: %v", e)
	}
	if e := explainKeyFailure("root@1.2.3.4: Permission denied (publickey,password)."); !strings.Contains(e.Error(), "拒绝了这把密钥") {
		t.Errorf("rejected key not explained: %v", e)
	}
	if e := explainKeyFailure("ssh: connect to host x port 22: Connection refused"); !strings.HasPrefix(e.Error(), "密钥登录失败") {
		t.Errorf("other error changed: %v", e)
	}
}
