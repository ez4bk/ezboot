package exp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
)

// KeyPair 结构体用于存储 RSA 密钥对
type KeyPair struct {
	PrivateKey *rsa.PrivateKey
	// PublicKey 是 PKIX DER 公钥的 base64 编码。
	PublicKey string
}

// GenerateKeyPair 生成新的 RSA 密钥对
func GenerateKeyPair() (*KeyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	publicKey, err := marshalPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	return &KeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// MarshalPrivateKey 将 KeyPair 的私钥序列化为 base64 编码的 PKCS#1 DER 字符串。
func MarshalPrivateKey(kp *KeyPair) (string, error) {
	if kp == nil || kp.PrivateKey == nil {
		return "", fmt.Errorf("rsa: private key is nil")
	}
	return base64.StdEncoding.EncodeToString(x509.MarshalPKCS1PrivateKey(kp.PrivateKey)), nil
}

// UnmarshalKeyPair 将 MarshalPrivateKey 输出的字符串反序列化为 KeyPair。
func UnmarshalKeyPair(encoded string) (*KeyPair, error) {
	derBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("rsa: base64 decode failed: %w", err)
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(derBytes)
	if err != nil {
		return nil, fmt.Errorf("rsa: parse private key failed: %w", err)
	}
	if err := privateKey.Validate(); err != nil {
		return nil, fmt.Errorf("rsa: invalid private key: %w", err)
	}

	publicKey, err := marshalPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	return &KeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// DecryptOAEP 使用 RSA-OAEP(SHA-256) 解密 base64 编码的密文。
func DecryptOAEP(keyPair *KeyPair, ciphertext string) ([]byte, error) {
	if err := validateKeyPair(keyPair); err != nil {
		return nil, err
	}

	encryptedBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("rsa: base64 decode failed: %w", err)
	}

	plaintext, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, keyPair.PrivateKey, encryptedBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("rsa: decrypt OAEP failed: %w", err)
	}
	return plaintext, nil
}

// DecryptPKCS1v15 使用 RSA PKCS#1 v1.5 解密 base64 编码的密文。
//
// 新代码应优先使用 DecryptOAEP；PKCS#1 v1.5 仅用于兼容已有数据。
func DecryptPKCS1v15(keyPair *KeyPair, ciphertext string) ([]byte, error) {
	if err := validateKeyPair(keyPair); err != nil {
		return nil, err
	}

	encryptedBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("rsa: base64 decode failed: %w", err)
	}

	plaintext, err := rsa.DecryptPKCS1v15(rand.Reader, keyPair.PrivateKey, encryptedBytes)
	if err != nil {
		return nil, fmt.Errorf("rsa: decrypt PKCS#1 v1.5 failed: %w", err)
	}
	return plaintext, nil
}

func validateKeyPair(kp *KeyPair) error {
	if kp == nil || kp.PrivateKey == nil {
		return fmt.Errorf("rsa: private key is nil")
	}
	return nil
}

func marshalPublicKey(publicKey *rsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("rsa: marshal public key failed: %w", err)
	}
	return base64.StdEncoding.EncodeToString(der), nil
}
