package service

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"strings"
)

func parseRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, err := decodePaymentPEM(raw, "PRIVATE KEY")
	if err != nil {
		return nil, err
	}
	if block.Type == "RSA PRIVATE KEY" {
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	private, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("私钥不是 RSA 密钥")
	}
	return private, nil
}

func parseRSAPublicKey(raw string) (*rsa.PublicKey, error) {
	block, err := decodePaymentPEM(raw, "PUBLIC KEY")
	if err != nil {
		return nil, err
	}
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		key, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("公钥不是 RSA 密钥")
		}
		return key, nil
	}
	return x509.ParsePKCS1PublicKey(block.Bytes)
}

func decodePaymentPEM(raw, kind string) (*pem.Block, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("缺少" + kind)
	}
	if !strings.Contains(raw, "BEGIN") {
		raw = wrapPaymentPEM(kind, raw)
	}
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, errors.New(kind + "格式错误")
	}
	return block, nil
}

func wrapPaymentPEM(kind, body string) string {
	compact := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\r', '\t':
			return -1
		default:
			return r
		}
	}, body)
	var b strings.Builder
	b.WriteString("-----BEGIN " + kind + "-----\n")
	for len(compact) > 64 {
		b.WriteString(compact[:64])
		b.WriteByte('\n')
		compact = compact[64:]
	}
	b.WriteString(compact)
	b.WriteString("\n-----END " + kind + "-----\n")
	return b.String()
}

func signRSA2(content string, key *rsa.PrivateKey) (string, error) {
	sum := sha256.Sum256([]byte(content))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func verifyRSA2(content, signature string, key *rsa.PublicKey) error {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return errors.New("签名格式错误")
	}
	sum := sha256.Sum256([]byte(content))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], raw); err != nil {
		return errors.New("签名校验失败")
	}
	return nil
}
