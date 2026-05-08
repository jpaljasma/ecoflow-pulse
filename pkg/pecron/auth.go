package pecron

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const randomAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateRandom returns the 16-character alphanumeric nonce expected by the
// Pecron mobile-cloud login flow.
func GenerateRandom() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate pecron login random: %w", err)
	}
	out := make([]byte, len(buf))
	for i, b := range buf {
		out[i] = randomAlphabet[int(b)%len(randomAlphabet)]
	}
	return string(out), nil
}

func deriveAESKey(randomValue string) string {
	sum := md5.Sum([]byte(randomValue))
	hexed := strings.ToUpper(hex.EncodeToString(sum[:]))
	return hexed[8:24]
}

// EncryptPassword applies the AES-CBC/PKCS7 password transform used by the
// Pecron Android app before email/password login.
func EncryptPassword(password string, randomValue string) (string, error) {
	key := deriveAESKey(randomValue)
	if len(key) != aes.BlockSize {
		return "", fmt.Errorf("invalid pecron AES key length %d", len(key))
	}
	iv := key[8:16] + key[0:8]
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", fmt.Errorf("init pecron password cipher: %w", err)
	}
	plain := pkcs7Pad([]byte(password), aes.BlockSize)
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, []byte(iv)).CryptBlocks(encrypted, plain)
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// BuildLoginSignature returns SHA-256(email + encrypted_password + random +
// userDomainSecret), matching the reverse-engineered mobile app request.
func BuildLoginSignature(email string, encryptedPassword string, randomValue string, secret string) string {
	sum := sha256.Sum256([]byte(email + encryptedPassword + randomValue + secret))
	return hex.EncodeToString(sum[:])
}

func pkcs7Pad(in []byte, blockSize int) []byte {
	if blockSize <= 0 {
		return append([]byte(nil), in...)
	}
	pad := blockSize - len(in)%blockSize
	if pad == 0 {
		pad = blockSize
	}
	return append(append([]byte(nil), in...), bytes.Repeat([]byte{byte(pad)}, pad)...)
}
