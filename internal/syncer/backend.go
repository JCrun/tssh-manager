package syncer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileName is the one file kept on the remote.
const FileName = "tssh-manager.sync.age"

var (
	errRemoteMissing = errors.New("remote file does not exist")
	errRemoteChanged = errors.New("remote file changed while syncing")
)

// Backend stores one opaque blob. Put must fail with errRemoteChanged when
// the remote is no longer at revision ifRev ("" = must not exist yet).
type Backend interface {
	Get(ctx context.Context) (blob []byte, rev string, err error)
	Put(ctx context.Context, blob []byte, ifRev string) error
	Describe() string
	Warnings() []string
}

// ---- folder: any directory another tool syncs (Syncthing, Dropbox, iCloud, OneDrive, NAS) ----

type folder struct{ dir string }

func (f folder) path() string     { return filepath.Join(f.dir, FileName) }
func (f folder) Describe() string { return "文件夹 " + f.dir }

func revOf(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func (f folder) Get(context.Context) ([]byte, string, error) {
	if st, err := os.Stat(f.dir); err != nil || !st.IsDir() {
		return nil, "", fmt.Errorf("同步文件夹不存在：%s", f.dir)
	}
	b, err := os.ReadFile(f.path())
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", errRemoteMissing
	}
	if err != nil {
		return nil, "", err
	}
	return b, revOf(b), nil
}

func (f folder) Put(_ context.Context, blob []byte, ifRev string) error {
	cur, err := os.ReadFile(f.path())
	switch {
	case errors.Is(err, os.ErrNotExist):
		if ifRev != "" {
			return errRemoteChanged
		}
	case err != nil:
		return err
	case revOf(cur) != ifRev:
		return errRemoteChanged
	}
	tmp, err := os.CreateTemp(f.dir, ".tssh-manager.sync-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(blob); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), f.path())
}

// Warnings reports conflict copies made by file sync tools, e.g.
// "tssh-manager.sync.sync-conflict-20260101-120000-ABC.age" (Syncthing) or
// "tssh-manager.sync (conflicted copy).age" (Dropbox).
func (f folder) Warnings() []string {
	m, _ := filepath.Glob(filepath.Join(f.dir, "tssh-manager.sync*"))
	var out []string
	for _, p := range m {
		n := filepath.Base(p)
		if n != FileName && strings.Contains(strings.ToLower(n), "conflict") {
			out = append(out, "同步文件夹里有同步工具生成的冲突副本："+n+"。它的内容没有被合并，确认无用后可删除。")
		}
	}
	return out
}

// ---- WebDAV: Nextcloud, 坚果云, Synology, any WebDAV server ----

type webdav struct {
	dir        string // collection URL, ends with "/"
	user, pass string
	client     *http.Client
}

func newWebDAV(rawURL, user, pass string) (*webdav, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, fmt.Errorf("WebDAV 地址无效：%q", rawURL)
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	return &webdav{dir: u.String(), user: user, pass: pass, client: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (w *webdav) Describe() string   { return "WebDAV " + w.dir }
func (w *webdav) Warnings() []string { return nil }
func (w *webdav) file() string       { return w.dir + FileName }

func (w *webdav) do(ctx context.Context, method, u string, body []byte, hdr map[string]string) (*http.Response, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, err
	}
	if w.user != "" || w.pass != "" {
		req.SetBasicAuth(w.user, w.pass)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return w.client.Do(req)
}

func httpErr(op string, r *http.Response) error {
	switch r.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("WebDAV %s：用户名或密码不对（HTTP %d）", op, r.StatusCode)
	}
	return fmt.Errorf("WebDAV %s 失败：HTTP %s", op, r.Status)
}

// rev prefers the ETag; servers without one fall back to a content hash,
// which Put checks with a fresh GET (a small race window, but no lost updates
// between two devices that sync minutes apart).
func (w *webdav) Get(ctx context.Context) ([]byte, string, error) {
	r, err := w.do(ctx, http.MethodGet, w.file(), nil, nil)
	if err != nil {
		return nil, "", err
	}
	defer r.Body.Close()
	if r.StatusCode == http.StatusNotFound {
		return nil, "", errRemoteMissing
	}
	if r.StatusCode != http.StatusOK {
		return nil, "", httpErr("读取", r)
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, 64<<20))
	if err != nil {
		return nil, "", err
	}
	if et := r.Header.Get("ETag"); et != "" {
		return b, "etag:" + et, nil
	}
	return b, "sha:" + revOf(b), nil
}

func (w *webdav) Put(ctx context.Context, blob []byte, ifRev string) error {
	hdr := map[string]string{"Content-Type": "application/octet-stream"}
	switch {
	case ifRev == "":
		hdr["If-None-Match"] = "*"
	case strings.HasPrefix(ifRev, "etag:"):
		hdr["If-Match"] = strings.TrimPrefix(ifRev, "etag:")
	default:
		b, _, err := w.Get(ctx)
		if err != nil && !errors.Is(err, errRemoteMissing) {
			return err
		}
		if err != nil || "sha:"+revOf(b) != ifRev {
			return errRemoteChanged
		}
	}
	for attempt := 0; ; attempt++ {
		r, err := w.do(ctx, http.MethodPut, w.file(), blob, hdr)
		if err != nil {
			return err
		}
		r.Body.Close()
		switch {
		case r.StatusCode >= 200 && r.StatusCode < 300:
			return nil
		case r.StatusCode == http.StatusPreconditionFailed:
			return errRemoteChanged
		case r.StatusCode == http.StatusConflict && attempt == 0:
			// parent collection missing: create it once
			m, err := w.do(ctx, "MKCOL", w.dir, nil, nil)
			if err != nil {
				return err
			}
			m.Body.Close()
			if m.StatusCode >= 300 && m.StatusCode != http.StatusMethodNotAllowed {
				return httpErr("创建目录", m)
			}
		default:
			return httpErr("上传", r)
		}
	}
}
