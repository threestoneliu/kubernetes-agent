package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	aead, err := NewAEAD(key)
	require.NoError(t, err)

	plain := []byte("hello kubeconfig content")
	blob, err := aead.Encrypt(plain)
	require.NoError(t, err)

	got, err := aead.Decrypt(blob)
	require.NoError(t, err)
	assert.Equal(t, plain, got)
}

func TestEncrypt_NonceUnique(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	aead, err := NewAEAD(key)
	require.NoError(t, err)

	a, _ := aead.Encrypt([]byte("same plaintext"))
	b, _ := aead.Encrypt([]byte("same plaintext"))
	assert.NotEqual(t, a, b, "nonce must randomize ciphertext")
}

func TestDecrypt_TamperedTag(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	aead, _ := NewAEAD(key)
	blob, _ := aead.Encrypt([]byte("x"))
	blob[len(blob)-1] ^= 0xff
	_, err := aead.Decrypt(blob)
	assert.Error(t, err)
}
