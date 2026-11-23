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
