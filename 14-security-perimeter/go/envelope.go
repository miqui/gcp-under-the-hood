package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"io"
)

// KMS is the minimal key-wrapping surface we need. The real implementation
// calls Cloud KMS Encrypt/Decrypt on the KEK; tests use a fake.
type KMS interface {
	Wrap(ctx context.Context, plaintextDEK []byte, aad []byte) ([]byte, error)
	Unwrap(ctx context.Context, wrappedDEK []byte, aad []byte) ([]byte, error)
}

// Envelope is the stored artifact: ciphertext + wrapped DEK. The KEK never leaves KMS.
type Envelope struct {
	KeyName    string `json:"key_name"`
	WrappedDEK []byte `json:"wrapped_dek"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

const dekSize = 32 // AES-256

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// Seal generates a fresh DEK, encrypts data locally with AES-GCM, wraps the DEK via KMS.
func Seal(ctx context.Context, k KMS, keyName string, plaintext, aad []byte, rnd io.Reader) (*Envelope, error) {
	dek := make([]byte, dekSize)
	defer zero(dek)
	if _, err := io.ReadFull(rnd, dek); err != nil {
		return nil, fmt.Errorf("generate dek: %w", err)
	}
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rnd, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	ct := gcm.Seal(nil, nonce, plaintext, aad)
	wrapped, err := k.Wrap(ctx, dek, aad)
	if err != nil {
		return nil, fmt.Errorf("wrap dek: %w", err)
	}
	return &Envelope{KeyName: keyName, WrappedDEK: wrapped, Nonce: nonce, Ciphertext: ct}, nil
}

// Open unwraps the DEK via KMS then decrypts locally.
func Open(ctx context.Context, k KMS, e *Envelope, aad []byte) ([]byte, error) {
	if e == nil {
		return nil, errors.New("nil envelope")
	}
	dek, err := k.Unwrap(ctx, e.WrappedDEK, aad)
	if err != nil {
		return nil, fmt.Errorf("unwrap dek: %w", err)
	}
	defer zero(dek)
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(e.Nonce) != gcm.NonceSize() {
		return nil, errors.New("bad nonce length")
	}
	pt, err := gcm.Open(nil, e.Nonce, e.Ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt data: %w", err)
	}
	return pt, nil
}

// Rewrap re-wraps the DEK under the key's current primary version without
// touching the (potentially huge) data ciphertext: the payoff of envelope encryption.
func Rewrap(ctx context.Context, k KMS, e *Envelope, aad []byte) (*Envelope, error) {
	dek, err := k.Unwrap(ctx, e.WrappedDEK, aad)
	if err != nil {
		return nil, fmt.Errorf("unwrap dek: %w", err)
	}
	defer zero(dek)
	w, err := k.Wrap(ctx, dek, aad)
	if err != nil {
		return nil, fmt.Errorf("rewrap dek: %w", err)
	}
	out := *e
	out.WrappedDEK = w
	return &out, nil
}
