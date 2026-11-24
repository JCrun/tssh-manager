package syncer

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"filippo.io/age"
)

// The remote file is an age file encrypted with a passphrase (scrypt), so it
// can also be opened by hand with "age -d tssh-manager.sync.age".

var workFactor = 17 // ~0.5 s and 128 MiB to derive the key

// ErrPassphrase means the remote file could not be decrypted with the passphrase.
var ErrPassphrase = errors.New("同步口令不对：远端文件无法用这个口令解密")

func seal(passphrase string, plain []byte) ([]byte, error) {
	r, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return nil, err
	}
	r.SetWorkFactor(workFactor)
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, r)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(plain); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func open(passphrase string, blob []byte) ([]byte, error) {
	id, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return nil, err
	}
	id.SetMaxWorkFactor(22)
	r, err := age.Decrypt(bytes.NewReader(blob), id)
	if err != nil {
		var nm *age.NoIdentityMatchError
		if errors.As(err, &nm) {
			return nil, ErrPassphrase
		}
		return nil, fmt.Errorf("远端文件无法解密：%w", err)
	}
	return io.ReadAll(io.LimitReader(r, 64<<20))
}
