package syncer

import (
	"tssh-manager/internal/sshconf"
)

// Conflict is one host block (or key file) changed differently on both sides
// since the last sync. Local/Remote are shown to the user with secrets masked;
// "" means that side deleted it.
type Conflict struct {
	ID     string `json:"id"`   // pass back in Options.Resolve
	Kind   string `json:"kind"` // host | block | key
	Name   string `json:"name"`
	Local  string `json:"local"`
	Remote string `json:"remote"`
	Note   string `json:"note"`
}

// Changes lists what one sync took from each side.
type Changes struct {
	FromRemote []string `json:"fromRemote"`
	FromLocal  []string `json:"fromLocal"`
}

type side int

const (
	same side = iota // both sides agree
	fromLocal
	fromRemote
	conflict
)

func eqp(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// pick3 is a three-way choice: a side that did not change since base yields
// to the side that did.
func pick3(b, l, r *string) (*string, side) {
	switch {
	case eqp(l, r):
		return l, same
	case eqp(l, b):
		return r, fromRemote
	case eqp(r, b):
		return l, fromLocal
	}
	return nil, conflict
}

func resolve(id string, l, r *string, res map[string]string) (*string, side, bool) {
	switch res[id] {
	case "local":
		return l, fromLocal, true
	case "remote":
		return r, fromRemote, true
	}
	return nil, conflict, false
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

type segIndex struct {
	order []string
	segs  map[string]sshconf.Segment
	tail  string
}

func index(text string) (segIndex, error) {
	segs, tail, err := sshconf.Segments(text)
	if err != nil {
		return segIndex{}, err
	}
	ix := segIndex{segs: map[string]sshconf.Segment{}, tail: tail}
	for _, s := range segs {
		ix.order = append(ix.order, s.Key)
		ix.segs[s.Key] = s
	}
	return ix, nil
}

func (ix segIndex) body(k string) *string {
	if s, ok := ix.segs[k]; ok {
		return &s.Body
	}
	return nil
}

func (ix segIndex) lead(k string) *string {
	if s, ok := ix.segs[k]; ok {
		return &s.Lead
	}
	return nil
}

// mergeConfig merges ssh configs block by block. A block is identified by its
// alias, so edits to different hosts never conflict. The comment/blank lines
// in front of a block (section headings) follow the block; if both sides
// changed them differently the local version is kept.
func mergeConfig(base, local, remote string, res map[string]string) (string, []Conflict, Changes, error) {
	var ch Changes
	B, err := index(base)
	if err != nil {
		return "", nil, ch, err
	}
	L, err := index(local)
	if err != nil {
		return "", nil, ch, err
	}
	R, err := index(remote)
	if err != nil {
		return "", nil, ch, err
	}
	type out struct{ lead, body string }
	kept := map[string]out{}
	var conflicts []Conflict
	keys := append(append(append([]string{}, L.order...), R.order...), B.order...)
	done := map[string]bool{}
	for _, k := range keys {
		if done[k] {
			continue
		}
		done[k] = true
		lb, rb := L.body(k), R.body(k)
		body, sd := pick3(B.body(k), lb, rb)
		if sd == conflict {
			id := "cfg:" + k
			var ok bool
			if body, sd, ok = resolve(id, lb, rb, res); !ok {
				name, kind := L.segs[k].Name, "host"
				if name == "" {
					name = R.segs[k].Name
				}
				if k[:5] == "block" {
					kind = "block"
				}
				cf := Conflict{ID: id, Kind: kind, Name: name, Local: sshconf.Redact(str(lb)), Remote: sshconf.Redact(str(rb))}
				if lb != nil && rb != nil && cf.Local == cf.Remote {
					cf.Note = "两边只有密码等保密值不同"
				}
				conflicts = append(conflicts, cf)
				continue
			}
		}
		name := L.segs[k].Name
		if name == "" {
			name = R.segs[k].Name
		}
		if name == "" {
			name = B.segs[k].Name
		}
		switch {
		case sd == fromRemote && !eqp(lb, rb):
			ch.FromRemote = append(ch.FromRemote, name)
		case sd == fromLocal && !eqp(lb, rb):
			ch.FromLocal = append(ch.FromLocal, name)
		}
		if body == nil {
			continue // deleted
		}
		lead, lsd := pick3(B.lead(k), L.lead(k), R.lead(k))
		if lsd == conflict || lead == nil {
			lead = L.lead(k)
			if sd == fromRemote || lead == nil {
				lead = R.lead(k)
			}
		}
		kept[k] = out{str(lead), *body}
	}
	if len(conflicts) > 0 {
		return "", conflicts, ch, nil
	}

	// order: local order, then remote-only blocks after their remote predecessor
	order := []string{}
	pos := map[string]bool{}
	for _, k := range L.order {
		if _, ok := kept[k]; ok {
			order = append(order, k)
			pos[k] = true
		}
	}
	insertAfter := func(prev, k string) {
		at := 0
		if prev != "" {
			for i, x := range order {
				if x == prev {
					at = i + 1
				}
			}
		}
		order = append(order[:at], append([]string{k}, order[at:]...)...)
		pos[k] = true
	}
	for _, list := range [][]string{R.order, B.order} {
		prev := ""
		for _, k := range list {
			if _, ok := kept[k]; ok && !pos[k] {
				insertAfter(prev, k)
			}
			if pos[k] {
				prev = k
			}
		}
	}
	tail, tsd := pick3(&B.tail, &L.tail, &R.tail)
	if tsd == conflict {
		tail = &L.tail
	}
	text := ""
	for _, k := range order {
		text += kept[k].lead + kept[k].body
	}
	text += *tail
	if _, err := sshconf.Parse(text); err != nil {
		return "", nil, ch, err
	}
	return text, nil, ch, nil
}

// mergeFiles merges key files by name.
func mergeFiles(base, local, remote map[string]string, res map[string]string) (map[string]string, []Conflict, Changes) {
	var ch Changes
	var conflicts []Conflict
	out := map[string]string{}
	get := func(m map[string]string, k string) *string {
		if v, ok := m[k]; ok {
			return &v
		}
		return nil
	}
	seen := map[string]bool{}
	for _, m := range []map[string]string{local, remote, base} {
		for k := range m {
			if seen[k] {
				continue
			}
			seen[k] = true
			l, r := get(local, k), get(remote, k)
			v, sd := pick3(get(base, k), l, r)
			if sd == conflict {
				id := "key:" + k
				var ok bool
				if v, sd, ok = resolve(id, l, r, res); !ok {
					desc := func(p *string) string {
						if p == nil {
							return ""
						}
						return "（密钥文件内容不显示）"
					}
					conflicts = append(conflicts, Conflict{ID: id, Kind: "key", Name: k, Local: desc(l), Remote: desc(r)})
					continue
				}
			}
			if !eqp(l, r) {
				if sd == fromRemote {
					ch.FromRemote = append(ch.FromRemote, "密钥 "+k)
				} else if sd == fromLocal {
					ch.FromLocal = append(ch.FromLocal, "密钥 "+k)
				}
			}
			if v != nil {
				out[k] = *v
			}
		}
	}
	return out, conflicts, ch
}
