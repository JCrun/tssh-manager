// Package sshconf reads and edits an OpenSSH client config (with tssh "#!!"
// extensions) at the line level, so that every line the user did not touch is
// written back byte-for-byte.
//
// Values are interpreted with github.com/trzsz/ssh_config, the parser tssh
// itself uses; this package only decides which lines to replace, insert or
// delete.
package sshconf

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	sshcfg "github.com/trzsz/ssh_config"
)

// Doc is a parsed config file.
type Doc struct {
	lines   []string // without trailing "\n"
	blocks  []block
	trailNL bool // original text ended with a newline
}

type block struct {
	start    int // index of the Host/Match line
	bodyEnd  int // one past the last line that belongs to this host (KV or #!! line)
	end      int // one past the last line before the next block (includes trailing comments/blank lines)
	match    bool
	patterns []string
	section  string
}

var (
	hostLineRe = regexp.MustCompile(`(?i)^\s*(host|match)(\s*=\s*|\s+)(.*)$`)
	aliasRe    = regexp.MustCompile(`^[A-Za-z0-9._@-]+$`)
)

// ValidAlias reports whether s is a safe single host alias (no wildcards, no spaces).
func ValidAlias(s string) bool { return aliasRe.MatchString(s) }

// Parse splits text into lines and host blocks.
func Parse(text string) (*Doc, error) {
	if _, err := sshcfg.DecodeBytes([]byte(text)); err != nil {
		return nil, fmt.Errorf("ssh config parse: %w", err)
	}
	d := &Doc{trailNL: strings.HasSuffix(text, "\n")}
	t := strings.TrimSuffix(text, "\n")
	if t != "" || text != "" {
		d.lines = strings.Split(t, "\n")
	}
	d.index()
	return d, nil
}

// line classification
type lineKind int

const (
	blank lineKind = iota
	comment
	ext // #!! key value
	kv
)

func classify(l string) (lineKind, string, string) {
	s := strings.TrimSpace(strings.TrimSuffix(l, "\r"))
	switch {
	case s == "":
		return blank, "", ""
	case strings.HasPrefix(s, "#!!"):
		k, v := splitKV(strings.TrimSpace(s[3:]))
		return ext, k, v
	case strings.HasPrefix(s, "#"):
		return comment, "", strings.TrimSpace(s[1:])
	default:
		k, v := splitKV(s)
		return kv, k, v
	}
}

func splitKV(s string) (string, string) {
	i := strings.IndexAny(s, " \t=")
	if i < 0 {
		return s, ""
	}
	k := s[:i]
	v := strings.TrimLeft(s[i:], " \t")
	v = strings.TrimPrefix(v, "=")
	return k, strings.TrimSpace(v)
}

func (d *Doc) index() {
	d.blocks = nil
	section := ""
	lastComment := "" // first line of the comment paragraph right above a Host line
	prev := blank
	for i, l := range d.lines {
		if m := hostLineRe.FindStringSubmatch(l); m != nil {
			if lastComment != "" {
				section = lastComment
			}
			lastComment, prev = "", kv
			b := block{start: i, bodyEnd: i + 1, end: len(d.lines), section: section,
				match: strings.EqualFold(m[1], "match")}
			rest := m[3]
			if j := strings.Index(rest, "#"); j >= 0 {
				rest = rest[:j]
			}
			b.patterns = strings.Fields(rest)
			if n := len(d.blocks); n > 0 {
				d.blocks[n-1].end = i
			}
			d.blocks = append(d.blocks, b)
			continue
		}
		kind, _, text := classify(l)
		switch kind {
		case kv, ext:
			lastComment = ""
			if n := len(d.blocks); n > 0 {
				d.blocks[n-1].bodyEnd = i + 1
			}
		case comment:
			if prev != comment {
				lastComment = text
			}
		}
		prev = kind
	}
}

// Entry is one Host block as shown to the user.
type Entry struct {
	Alias     string            `json:"alias"`    // first pattern
	Patterns  []string          `json:"patterns"` // all patterns on the Host line
	Wildcard  bool              `json:"wildcard"` // any pattern has * ? ! or this is a Match block
	Section   string            `json:"section"`  // nearest preceding "# comment" heading
	Line      int               `json:"line"`     // 1-based line of the Host line
	Options   []Option          `json:"options"`  // lines in the block, in file order
	Effective map[string]string `json:"effective"`
	Raw       string            `json:"raw"`
}

// Option is one key/value line inside a block.
type Option struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Ext   bool   `json:"ext"` // #!! line
}

// effectiveKeys are resolved through trzsz/ssh_config (includes Host * defaults).
var effectiveKeys = []string{"HostName", "Port", "User", "IdentityFile", "ProxyJump", "ProxyCommand", "IdentitiesOnly"}

// Entries returns every Host/Match block.
func (d *Doc) Entries() []Entry {
	cfg, _ := sshcfg.DecodeBytes([]byte(d.String()))
	out := make([]Entry, 0, len(d.blocks))
	for _, b := range d.blocks {
		e := Entry{Patterns: b.patterns, Section: b.section, Line: b.start + 1, Wildcard: b.match,
			Raw: strings.Join(d.lines[b.start:b.bodyEnd], "\n")}
		if len(b.patterns) > 0 {
			e.Alias = b.patterns[0]
		}
		for _, p := range b.patterns {
			if strings.ContainsAny(p, "*?!") {
				e.Wildcard = true
			}
		}
		for _, l := range d.lines[b.start+1 : b.bodyEnd] {
			kind, k, v := classify(l)
			if kind == kv || kind == ext {
				e.Options = append(e.Options, Option{Key: k, Value: v, Ext: kind == ext})
			}
		}
		if !e.Wildcard && cfg != nil && len(b.patterns) == 1 {
			e.Effective = map[string]string{}
			for _, k := range effectiveKeys {
				if v, err := cfg.Get(e.Alias, k); err == nil && v != "" {
					e.Effective[k] = v
				}
			}
		}
		out = append(out, e)
	}
	return out
}

func (d *Doc) String() string {
	s := strings.Join(d.lines, "\n")
	if d.trailNL && len(d.lines) > 0 {
		s += "\n"
	}
	return s
}

// ErrNotFound is returned when an alias has no single-pattern Host block.
var ErrNotFound = errors.New("host not found")

func (d *Doc) find(alias string) (int, error) {
	idx := -1
	for i, b := range d.blocks {
		if !b.match && len(b.patterns) == 1 && b.patterns[0] == alias {
			if idx >= 0 {
				return -1, fmt.Errorf("host %q is defined more than once", alias)
			}
			idx = i
		}
	}
	if idx < 0 {
		return -1, ErrNotFound
	}
	return idx, nil
}

// Change sets one option. Empty Value deletes it. Ext selects a "#!!" line.
type Change struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Ext   bool   `json:"ext"`
}

func validateChange(c Change) error {
	if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`).MatchString(c.Key) {
		return fmt.Errorf("invalid option name %q", c.Key)
	}
	if strings.ContainsAny(c.Value, "\n\r") {
		return fmt.Errorf("option %s: value must be a single line", c.Key)
	}
	if strings.EqualFold(c.Key, "Host") || strings.EqualFold(c.Key, "Match") || strings.EqualFold(c.Key, "Include") {
		return fmt.Errorf("option %s cannot be set here", c.Key)
	}
	return nil
}

const indent = "    "

// Apply changes options of an existing host in place. Existing lines keep their
// position and indentation; new options are appended to the end of the block.
func (d *Doc) Apply(alias string, changes []Change) error {
	bi, err := d.find(alias)
	if err != nil {
		return err
	}
	for _, c := range changes {
		if err := validateChange(c); err != nil {
			return err
		}
	}
	for _, c := range changes {
		b := d.blocks[bi]
		found := -1
		for i := b.start + 1; i < b.bodyEnd; i++ {
			kind, k, _ := classify(d.lines[i])
			if ((c.Ext && kind == ext) || (!c.Ext && kind == kv)) && strings.EqualFold(k, c.Key) {
				found = i
				break
			}
		}
		switch {
		case found >= 0 && c.Value == "":
			d.lines = append(d.lines[:found], d.lines[found+1:]...)
		case found >= 0:
			d.lines[found] = rewriteValue(d.lines[found], c)
		case c.Value != "":
			ind := blockIndent(d.lines[b.start+1:b.bodyEnd])
			line := ind + c.Key + " " + c.Value
			if c.Ext {
				line = ind + "#!! " + c.Key + " " + c.Value
			}
			at := b.bodyEnd
			d.lines = append(d.lines[:at], append([]string{line}, d.lines[at:]...)...)
		}
		d.index()
	}
	return nil
}

// rewriteValue keeps the original indentation, key spelling and separator.
func rewriteValue(l string, c Change) string {
	lead := l[:len(l)-len(strings.TrimLeft(l, " \t"))]
	s := strings.TrimLeft(l, " \t")
	prefix := ""
	if c.Ext {
		prefix = "#!! "
		s = strings.TrimSpace(s[3:])
	}
	k, _ := splitKV(s)
	return lead + prefix + k + " " + c.Value
}

func blockIndent(body []string) string {
	for _, l := range body {
		if kind, _, _ := classify(l); kind == kv || kind == ext {
			return l[:len(l)-len(strings.TrimLeft(l, " \t"))]
		}
	}
	return indent
}

// Add appends a new Host block. If section names an existing heading, the block
// goes after the last host of that section; otherwise it goes at the end, under
// a new "# section" heading when section is not empty.
func (d *Doc) Add(alias, section string, changes []Change) error {
	if !ValidAlias(alias) {
		return fmt.Errorf("invalid host alias %q", alias)
	}
	if _, err := d.find(alias); err == nil {
		return fmt.Errorf("host %q already exists", alias)
	}
	for _, c := range changes {
		if err := validateChange(c); err != nil {
			return err
		}
	}
	section = strings.TrimSpace(strings.ReplaceAll(section, "\n", " "))
	blk := []string{"Host " + alias}
	for _, c := range changes {
		if c.Value == "" {
			continue
		}
		if c.Ext {
			blk = append(blk, indent+"#!! "+c.Key+" "+c.Value)
		} else {
			blk = append(blk, indent+c.Key+" "+c.Value)
		}
	}
	at, heading := len(d.lines), section != ""
	for _, b := range d.blocks {
		if section != "" && b.section == section {
			at, heading = b.bodyEnd, false
		}
	}
	// trim trailing blank lines at the end of the file before appending there
	if at == len(d.lines) {
		for at > 0 && strings.TrimSpace(d.lines[at-1]) == "" {
			at--
		}
	}
	ins := []string{}
	if at > 0 {
		ins = append(ins, "")
	}
	if heading {
		ins = append(ins, "# "+section)
	}
	ins = append(ins, blk...)
	tail := append([]string{}, d.lines[at:]...)
	d.lines = append(append(d.lines[:at], ins...), tail...)
	if len(tail) == 0 {
		d.trailNL = true
	}
	d.index()
	return nil
}

// Delete removes a host block and one blank line after it. Section headings and
// comments that introduce the next block stay.
func (d *Doc) Delete(alias string) error {
	bi, err := d.find(alias)
	if err != nil {
		return err
	}
	b := d.blocks[bi]
	from, to := b.start, b.bodyEnd
	// own leading comment lines directly above the Host line belong to the block
	// only if they are not a section heading for a following host.
	if to < len(d.lines) && strings.TrimSpace(d.lines[to]) == "" {
		to++
	} else if from > 0 && strings.TrimSpace(d.lines[from-1]) == "" {
		from--
	}
	d.lines = append(d.lines[:from], d.lines[to:]...)
	d.index()
	return nil
}

// Rename changes the alias on the Host line.
func (d *Doc) Rename(alias, to string) error {
	if !ValidAlias(to) {
		return fmt.Errorf("invalid host alias %q", to)
	}
	if _, err := d.find(to); err == nil {
		return fmt.Errorf("host %q already exists", to)
	}
	bi, err := d.find(alias)
	if err != nil {
		return err
	}
	l := d.lines[d.blocks[bi].start]
	m := hostLineRe.FindStringSubmatchIndex(l)
	rest := l[m[6]:]
	d.lines[d.blocks[bi].start] = l[:m[6]] + strings.Replace(rest, alias, to, 1)
	d.index()
	return nil
}

// ReplaceRaw replaces a host block (Host line through its last option) with raw
// text, which must itself be exactly one Host block for the same or a new alias.
func (d *Doc) ReplaceRaw(alias, raw string) error {
	bi, err := d.find(alias)
	if err != nil {
		return err
	}
	raw = strings.TrimRight(strings.ReplaceAll(raw, "\r\n", "\n"), "\n ")
	nd, err := Parse(raw + "\n")
	if err != nil {
		return err
	}
	if len(nd.blocks) != 1 || nd.blocks[0].start != 0 || nd.blocks[0].match || len(nd.blocks[0].patterns) == 0 {
		return errors.New("raw text must be exactly one Host block starting with a Host line")
	}
	if nd.blocks[0].bodyEnd != len(nd.lines) {
		return errors.New("raw text must not end with comment lines")
	}
	newAlias := nd.blocks[0].patterns[0]
	if newAlias != alias {
		if _, err := d.find(newAlias); err == nil {
			return fmt.Errorf("host %q already exists", newAlias)
		}
	}
	b := d.blocks[bi]
	tail := append([]string{}, d.lines[b.bodyEnd:]...)
	d.lines = append(append(d.lines[:b.start], nd.lines...), tail...)
	d.index()
	return nil
}

// Sections lists section headings in file order.
func (d *Doc) Sections() []string {
	seen, out := map[string]bool{}, []string{}
	for _, b := range d.blocks {
		if b.section != "" && !seen[b.section] {
			seen[b.section] = true
			out = append(out, b.section)
		}
	}
	return out
}

// Validate re-parses the document with the tssh parser.
func (d *Doc) Validate() error {
	_, err := sshcfg.DecodeBytes([]byte(d.String()))
	return err
}

// Lookup resolves key for alias the way tssh does (first match, quotes removed).
func (d *Doc) Lookup(alias, key string) (string, error) {
	cfg, err := sshcfg.DecodeBytes([]byte(d.String()))
	if err != nil {
		return "", err
	}
	return cfg.Get(alias, key)
}

// LocalValue returns the raw value of an option written in alias's own block.
func (d *Doc) LocalValue(alias, key string, isExt bool) (string, bool) {
	bi, err := d.find(alias)
	if err != nil {
		return "", false
	}
	b := d.blocks[bi]
	for i := b.start + 1; i < b.bodyEnd; i++ {
		kind, k, v := classify(d.lines[i])
		if ((isExt && kind == ext) || (!isExt && kind == kv)) && strings.EqualFold(k, key) {
			return v, true
		}
	}
	return "", false
}
