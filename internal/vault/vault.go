package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	Version   uint32 = 1
	NonceSize        = 12
)

type Ciphertext struct {
	Version int
	Nonce   []byte
	Data    []byte
}

type Vault struct{ aead cipher.AEAD }

func New(masterKey []byte) (*Vault, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("master key AES-256 harus 32 byte")
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Vault{aead: aead}, nil
}

func (v *Vault) Encrypt(plaintext, context []byte) (Ciphertext, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return Ciphertext{}, err
	}
	aad := make([]byte, 4+len(context))
	binary.BigEndian.PutUint32(aad, Version)
	copy(aad[4:], context)
	return Ciphertext{Version: int(Version), Nonce: nonce, Data: v.aead.Seal(nil, nonce, plaintext, aad)}, nil
}

func (v *Vault) Decrypt(c Ciphertext, context []byte) ([]byte, error) {
	if c.Version != int(Version) {
		return nil, fmt.Errorf("versi ciphertext tidak didukung: %d", c.Version)
	}
	if len(c.Nonce) != v.aead.NonceSize() {
		return nil, errors.New("nonce ciphertext tidak valid")
	}
	aad := make([]byte, 4+len(context))
	binary.BigEndian.PutUint32(aad, Version)
	copy(aad[4:], context)
	plain, err := v.aead.Open(nil, c.Nonce, c.Data, aad)
	if err != nil {
		return nil, errors.New("ciphertext tidak dapat diverifikasi")
	}
	return plain, nil
}
