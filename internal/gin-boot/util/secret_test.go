package util

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	const key = "test-secret"
	cipherText, err := Encrypt("user:password", key)
	if err != nil {
		t.Fatalf("Encrypt() = %v", err)
	}
	if strings.Contains(cipherText, "password") {
		t.Fatal("ciphertext contains plaintext")
	}

	plainText, err := Decrypt(cipherText, key)
	if err != nil {
		t.Fatalf("Decrypt() = %v", err)
	}
	if plainText != "user:password" {
		t.Fatalf("Decrypt() = %q, want %q", plainText, "user:password")
	}
}

func TestEncryptUsesRandomNonce(t *testing.T) {
	const key = "test-secret"
	first, err := Encrypt("same", key)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Encrypt("same", key)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("two encryptions produced the same ciphertext")
	}
}

func TestDecryptRejectsWrongKeyAndTampering(t *testing.T) {
	cipherText, err := Encrypt("secret", "key-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(cipherText, "key-b"); err == nil {
		t.Fatal("Decrypt() with wrong key succeeded")
	}

	raw, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0xff
	if _, err := Decrypt(base64.StdEncoding.EncodeToString(raw), "key-a"); err == nil {
		t.Fatal("Decrypt() with tampered ciphertext succeeded")
	}
}

func TestEncryptRejectsEmptyKey(t *testing.T) {
	if _, err := Encrypt("x", ""); err == nil {
		t.Fatal("Encrypt() with empty key succeeded")
	}
}
