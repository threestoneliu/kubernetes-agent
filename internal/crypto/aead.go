package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

type AEAD struct {
	gcm cipher.AEAD
}

func NewAEAD(key []byte) (*AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AEAD{gcm: gcm}, nil
}

const (
	nonceSize = 12
	tagSize   = 16
)

func (a *AEAD) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := a.gcm.Seal(nil, nonce, plaintext, nil)
	blob := make([]byte, 0, nonceSize+len(ct))
	blob = append(blob, nonce...)
	blob = append(blob, ct...)
	return blob, nil
}

func (a *AEAD) Decrypt(blob []byte) ([]byte, error) {
	if len(blob) < nonceSize+tagSize {
		return nil, errors.New("blob too short")
	}
	nonce := blob[:nonceSize]
	ct := blob[nonceSize:]
	return a.gcm.Open(nil, nonce, ct, nil)
}
