package exp

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() = %v", err)
	}
	if kp.PrivateKey == nil {
		t.Fatal("private key is nil")
	}
	if kp.PublicKey == "" {
		t.Fatal("public key is empty")
	}

	der, err := base64.StdEncoding.DecodeString(kp.PublicKey)
	if err != nil {
		t.Fatalf("public key is not valid base64: %v", err)
	}
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		t.Fatalf("public key is not valid PKIX DER: %v", err)
	}
	if _, ok := pub.(*rsa.PublicKey); !ok {
		t.Fatalf("public key type = %T, want *rsa.PublicKey", pub)
	}
}

func TestMarshalUnmarshalKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := MarshalPrivateKey(kp)
	if err != nil {
		t.Fatalf("MarshalPrivateKey() = %v", err)
	}
	if encoded == "" {
		t.Fatal("MarshalPrivateKey() returned empty string")
	}

	decoded, err := UnmarshalKeyPair(encoded)
	if err != nil {
		t.Fatalf("UnmarshalKeyPair() = %v", err)
	}
	if decoded.PrivateKey.N.Cmp(kp.PrivateKey.N) != 0 {
		t.Fatal("private key modulus changed after round trip")
	}
	if decoded.PrivateKey.E != kp.PrivateKey.E {
		t.Fatal("private key exponent changed after round trip")
	}
	if decoded.PublicKey != kp.PublicKey {
		t.Fatalf("public key = %q, want %q", decoded.PublicKey, kp.PublicKey)
	}
}

func TestMarshalPrivateKeyRejectsNil(t *testing.T) {
	if _, err := MarshalPrivateKey(nil); err == nil {
		t.Fatal("MarshalPrivateKey(nil) succeeded")
	}
	if _, err := MarshalPrivateKey(&KeyPair{}); err == nil {
		t.Fatal("MarshalPrivateKey(empty key) succeeded")
	}
}

func TestDecryptOAEP(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	const message = "hello rsa"
	encrypted, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &kp.PrivateKey.PublicKey, []byte(message), nil)
	if err != nil {
		t.Fatal(err)
	}

	plaintext, err := DecryptOAEP(kp, base64.StdEncoding.EncodeToString(encrypted))
	if err != nil {
		t.Fatalf("DecryptOAEP() = %v", err)
	}
	if !bytes.Equal(plaintext, []byte(message)) {
		t.Fatalf("plaintext = %q, want %q", plaintext, message)
	}
}

func TestDecryptPKCS1v15(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	const message = "legacy rsa"
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, &kp.PrivateKey.PublicKey, []byte(message))
	if err != nil {
		t.Fatal(err)
	}

	plaintext, err := DecryptPKCS1v15(kp, base64.StdEncoding.EncodeToString(encrypted))
	if err != nil {
		t.Fatalf("DecryptPKCS1v15() = %v", err)
	}
	if !bytes.Equal(plaintext, []byte(message)) {
		t.Fatalf("plaintext = %q, want %q", plaintext, message)
	}
}

func TestDecryptRejectsNilKeyAndBadBase64(t *testing.T) {
	if _, err := DecryptOAEP(nil, "AAAA"); err == nil {
		t.Fatal("DecryptOAEP(nil) succeeded")
	}
	if _, err := DecryptPKCS1v15(&KeyPair{}, "AAAA"); err == nil {
		t.Fatal("DecryptPKCS1v15(empty key) succeeded")
	}

	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptOAEP(kp, "not-base64"); err == nil {
		t.Fatal("DecryptOAEP(invalid base64) succeeded")
	}
}
