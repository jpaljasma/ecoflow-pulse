package ankersolix

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	ankerServerPublicKeyHex = "04c5c00c4f8d1197cc7c3167c52bf7acb054d722f0ef08dcd7e0883236e0d72a3868d9750cb47fa4619248f3d83f0f662671dadc6e2d31c2f41db0161651c7c076"
	aesBlockSize            = aes.BlockSize
)

type PasswordEncryptionInput struct {
	Password        string
	ClientPrivate   *ecdh.PrivateKey
	Random          io.Reader
	ServerPublicKey []byte
}

type EncryptedPassword struct {
	CiphertextBase64   string
	ClientPublicKeyHex string
}

func EncryptPassword(input PasswordEncryptionInput) (EncryptedPassword, error) {
	serverPublic := input.ServerPublicKey
	if len(serverPublic) == 0 {
		decoded, err := hex.DecodeString(ankerServerPublicKeyHex)
		if err != nil {
			return EncryptedPassword{}, err
		}
		serverPublic = decoded
	}
	peer, err := ecdh.P256().NewPublicKey(serverPublic)
	if err != nil {
		return EncryptedPassword{}, fmt.Errorf("parse anker server public key: %w", err)
	}
	private := input.ClientPrivate
	if private == nil {
		reader := input.Random
		if reader == nil {
			reader = rand.Reader
		}
		private, err = ecdh.P256().GenerateKey(reader)
		if err != nil {
			return EncryptedPassword{}, fmt.Errorf("generate anker client key: %w", err)
		}
	}
	shared, err := private.ECDH(peer)
	if err != nil {
		return EncryptedPassword{}, fmt.Errorf("derive anker login secret: %w", err)
	}
	block, err := aes.NewCipher(shared)
	if err != nil {
		return EncryptedPassword{}, fmt.Errorf("create anker login cipher: %w", err)
	}
	plain := pkcs7Pad([]byte(input.Password), aesBlockSize)
	cipher.NewCBCEncrypter(block, shared[:aesBlockSize]).CryptBlocks(plain, plain)
	return EncryptedPassword{
		CiphertextBase64:   base64.StdEncoding.EncodeToString(plain),
		ClientPublicKeyHex: hex.EncodeToString(private.PublicKey().Bytes()),
	}, nil
}

func pkcs7Pad(in []byte, blockSize int) []byte {
	if blockSize <= 0 {
		return append([]byte(nil), in...)
	}
	padding := blockSize - len(in)%blockSize
	out := append([]byte(nil), in...)
	for i := 0; i < padding; i++ {
		out = append(out, byte(padding))
	}
	return out
}

func pkcs7Unpad(in []byte, blockSize int) ([]byte, error) {
	if len(in) == 0 || blockSize <= 0 || len(in)%blockSize != 0 {
		return nil, errors.New("invalid PKCS7 block")
	}
	padding := int(in[len(in)-1])
	if padding == 0 || padding > blockSize || padding > len(in) {
		return nil, errors.New("invalid PKCS7 padding")
	}
	for _, b := range in[len(in)-padding:] {
		if int(b) != padding {
			return nil, errors.New("invalid PKCS7 padding")
		}
	}
	return append([]byte(nil), in[:len(in)-padding]...), nil
}

type Session struct {
	UserID         string
	AuthToken      string
	Nickname       string
	TokenExpiresAt int64
}

func TokenHeaders(cfg Config, session Session, timezone string) http.Header {
	cfg = normalizeConfig(cfg)
	headers := http.Header{}
	headers.Set("content-type", "application/json")
	headers.Set("model-type", "DESKTOP")
	headers.Set("app-name", "anker_power")
	headers.Set("os-type", "android")
	if country := strings.ToUpper(strings.TrimSpace(cfg.Country)); country != "" {
		headers.Set("country", country)
	}
	if strings.TrimSpace(timezone) != "" {
		headers.Set("timezone", strings.TrimSpace(timezone))
	}
	if strings.TrimSpace(session.UserID) != "" {
		headers.Set("gtoken", md5Hex(strings.TrimSpace(session.UserID)))
	}
	if strings.TrimSpace(session.AuthToken) != "" {
		headers.Set("x-auth-token", strings.TrimSpace(session.AuthToken))
	}
	return headers
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}
