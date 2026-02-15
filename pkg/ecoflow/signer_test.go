package ecoflow

import (
	"context"
	"net/url"
	"testing"
)

func TestHMACSHA256Signer_Sign(t *testing.T) {
	t.Parallel()

	signer := NewHMACSHA256Signer()
	result, err := signer.Sign(context.Background(), SignInput{
		Credentials: Credentials{
			AccessKey: "AKID",
			SecretKey: "SECRET",
		},
		Nonce:           "nonce-123",
		TimestampMillis: 1700000000000,
		Query: url.Values{
			"sn":   {"ECO-1"},
			"page": {"1"},
		},
		Body: map[string]any{
			"cmdCode": "getInfo",
			"size":    20,
		},
	})
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	const expectedCanonical = "cmdCode=getInfo&page=1&size=20&sn=ECO-1&accessKey=AKID&nonce=nonce-123&timestamp=1700000000000"
	if result.Canonical != expectedCanonical {
		t.Fatalf("canonical mismatch\nwant: %s\ngot:  %s", expectedCanonical, result.Canonical)
	}

	const expectedSignature = "2bcbbcc56c37274ed28be7ad5980b62a8cd47dd502656060abce2e89565b7562"
	if result.Signature != expectedSignature {
		t.Fatalf("signature mismatch\nwant: %s\ngot:  %s", expectedSignature, result.Signature)
	}
}
