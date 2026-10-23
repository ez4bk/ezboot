// Copyright (C) 2019 LambdaCal Inc.

package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// Encrypt encrypts plainText with AES-256-GCM.
//
// The key is an application secret; it must be provided by the caller (for
// example from an environment variable) and must not be hard-coded. The key
// material is derived with SHA-256 so callers can use an arbitrary-length
// secret. A fresh nonce is generated for every call and prepended to the
// ciphertext.
func Encrypt(plainText, key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("secret: encryption key is empty")
	}

	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("secret: generate nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a value produced by Encrypt.
func Decrypt(cipherText, key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("secret: encryption key is empty")
	}

	raw, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", fmt.Errorf("secret: decode ciphertext: %w", err)
	}

	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("secret: ciphertext is too short")
	}

	nonce, data := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", fmt.Errorf("secret: decrypt: %w", err)
	}
	return string(plain), nil
}

func newGCM(key string) (cipher.AEAD, error) {
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, fmt.Errorf("secret: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secret: create GCM: %w", err)
	}
	return gcm, nil
}

// Base64Encode encodes bytes as standard base64.
func Base64Encode(input []byte) string {
	return base64.StdEncoding.EncodeToString(input)
}

// Base64Decode decodes standard base64.
func Base64Decode(input string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(input)
}
