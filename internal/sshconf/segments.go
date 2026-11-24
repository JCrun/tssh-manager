package sshconf

import (
	"fmt"
	"regexp"
	"strings"
)

// Segment is one Host/Match block for merging: Lead holds the blank and
// comment lines between the previous block and this Host line (section
// headings live here), Body the Host line through its last option. Both end
// with "\n" when not empty, so joining all Lead+Body plus the tail rebuilds
// the file.
type Segment struct {
	Key  string // "host:<alias>" for single-pattern hosts, else "block:<Host line>"; "#n" added for repeats
	Name string // what to show the user
	Lead string
	Body string
}

func joinLines(ls []string) string {
	if len(ls) == 0 {
		return ""
	}
	return strings.Join(ls, "\n") + "\n"
}

// Segments splits text into blocks plus the lines after the last block.
func Segments(text string) ([]Segment, string, error) {
	d, err := Parse(text)
	if err != nil {
		return nil, "", err
	}
	var out []Segment
	seen := map[string]int{}
	prev := 0
	for _, b := range d.blocks {
		hostLine := strings.TrimSpace(d.lines[b.start])
		key, name := "block:"+strings.Join(strings.Fields(hostLine), " "), hostLine
		if !b.match && len(b.patterns) == 1 {
			key, name = "host:"+b.patterns[0], b.patterns[0]
		}
		seen[key]++
		if n := seen[key]; n > 1 {
			key = fmt.Sprintf("%s#%d", key, n)
		}
		out = append(out, Segment{Key: key, Name: name,
			Lead: joinLines(d.lines[prev:b.start]), Body: joinLines(d.lines[b.start:b.bodyEnd])})
		prev = b.bodyEnd
	}
	return out, joinLines(d.lines[prev:]), nil
}

// Normalize returns text the way Segments rebuilds it (ending in a newline).
func Normalize(text string) string {
	if text == "" || strings.HasSuffix(text, "\n") {
		return text
	}
	return text + "\n"
}

var sensitiveKeyRe = regexp.MustCompile(`(?i)(password|passphrase|secret|otp)`)

// Mask replaces secret values shown to the user.
const Mask = "******"

// SensitiveKey reports whether an option holds a secret.
func SensitiveKey(k string) bool { return sensitiveKeyRe.MatchString(k) }

// RedactLine masks the value of password/secret options in one config line.
func RedactLine(l string) string {
	t := strings.TrimSpace(l)
	isExt := strings.HasPrefix(t, "#!!")
	if strings.HasPrefix(t, "#") && !isExt {
		return l
	}
	body := t
	if isExt {
		body = strings.TrimSpace(t[3:])
	}
	k, v := splitKV(body)
	if v == "" || !SensitiveKey(k) {
		return l
	}
	return l[:strings.Index(l, k)+len(k)] + " " + Mask
}

// Redact masks secrets in every line of text.
func Redact(text string) string {
	ls := strings.Split(text, "\n")
	for i, l := range ls {
		ls[i] = RedactLine(l)
	}
	return strings.Join(ls, "\n")
}
