package grpcmw

import (
	"context"
	"errors"
	"testing"
)

type fakeClaimsVerifier struct {
	claims *tokenClaims
	err    error
}

func (f fakeClaimsVerifier) Verify(context.Context, string) (*tokenClaims, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.claims, nil
}

func TestKeycloakAuthorizerRejectsMissingToken(t *testing.T) {
	t.Parallel()

	a := &KeycloakJWKSAuthorizer{
		verifier:        fakeClaimsVerifier{},
		allowMissingJWT: false,
	}
	err := a.Authorize(context.Background(), "/pulse.test/Foo", &Claims{})
	if err == nil {
		t.Fatalf("expected error for missing token")
	}
}

func TestKeycloakAuthorizerAllowsMissingTokenWhenConfigured(t *testing.T) {
	t.Parallel()

	a := &KeycloakJWKSAuthorizer{
		verifier:        fakeClaimsVerifier{},
		allowMissingJWT: true,
	}
	if err := a.Authorize(context.Background(), "/pulse.test/Foo", &Claims{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKeycloakAuthorizerPopulatesClaims(t *testing.T) {
	t.Parallel()

	a := &KeycloakJWKSAuthorizer{
		verifier: fakeClaimsVerifier{
			claims: &tokenClaims{
				Subject: "user-1",
				Email:   "u1@example.com",
				RealmAccess: roleSet{
					Roles: []string{"admin", "viewer"},
				},
				ResourceAccess: map[string]roleSet{
					"pulse-api": {Roles: []string{"viewer", "ops"}},
				},
			},
		},
	}
	claims := &Claims{RawJWT: "jwt-token"}
	if err := a.Authorize(context.Background(), "/pulse.test/Foo", claims); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("unexpected subject: %q", claims.Subject)
	}
	if claims.Email != "u1@example.com" {
		t.Fatalf("unexpected email: %q", claims.Email)
	}
	if len(claims.Roles) != 3 {
		t.Fatalf("unexpected roles length: %d", len(claims.Roles))
	}
}

func TestKeycloakAuthorizerVerifierFailure(t *testing.T) {
	t.Parallel()

	a := &KeycloakJWKSAuthorizer{
		verifier: fakeClaimsVerifier{err: errors.New("verify failed")},
	}
	err := a.Authorize(context.Background(), "/pulse.test/Foo", &Claims{RawJWT: "jwt-token"})
	if err == nil {
		t.Fatalf("expected verifier error")
	}
}
