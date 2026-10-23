package exp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// Encrypt 使用 AES-256-GCM 加密明文。
// masterKey 必须是 32 字节。
// 返回值格式: nonce(12 bytes) + ciphertext + tag(16 bytes)
func Encrypt(masterKey, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: read nonce: %w", err)
	}

	// Seal appends ciphertext+tag after nonce
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt 使用 AES-256-GCM 解密密文。
// masterKey 必须是 32 字节。
// ciphertext 格式: nonce(12 bytes) + encrypted data + tag(16 bytes)
func Decrypt(masterKey, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("crypto: ciphertext too short")
	}

	nonce, encrypted := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypt: %w", err)
	}

	return plaintext, nil
}
