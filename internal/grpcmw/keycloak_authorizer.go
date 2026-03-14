package grpcmw

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	oidc "github.com/coreos/go-oidc/v3/oidc"
)

type KeycloakJWKSAuthorizerConfig struct {
	IssuerURL       string
	Audience        string
	JWKSURL         string
	AllowMissingJWT bool
}

type tokenClaims struct {
	Subject          string             `json:"sub"`
	Email            string             `json:"email"`
	EmailVerified    bool               `json:"email_verified"`
	DisplayName      string             `json:"name"`
	GivenName        string             `json:"given_name"`
	FamilyName       string             `json:"family_name"`
	Picture          string             `json:"picture"`
	Locale           string             `json:"locale"`
	IdentityProvider string             `json:"identity_provider"`
	RealmAccess      roleSet            `json:"realm_access"`
	ResourceAccess   map[string]roleSet `json:"resource_access"`
}

type roleSet struct {
	Roles []string `json:"roles"`
}

type claimsVerifier interface {
	Verify(ctx context.Context, rawJWT string) (*tokenClaims, error)
}

type KeycloakJWKSAuthorizer struct {
	verifier        claimsVerifier
	allowMissingJWT bool
}

func NewKeycloakJWKSAuthorizer(ctx context.Context, cfg KeycloakJWKSAuthorizerConfig) (*KeycloakJWKSAuthorizer, error) {
	issuer := strings.TrimSpace(cfg.IssuerURL)
	if issuer == "" {
		return nil, errors.New("keycloak issuer URL is required")
	}
	verifierCfg := &oidc.Config{SkipClientIDCheck: true}
	if strings.TrimSpace(cfg.Audience) != "" {
		verifierCfg.ClientID = strings.TrimSpace(cfg.Audience)
		verifierCfg.SkipClientIDCheck = false
	}

	var verifier *oidc.IDTokenVerifier
	if jwksURL := strings.TrimSpace(cfg.JWKSURL); jwksURL != "" {
		verifier = oidc.NewVerifier(issuer, oidc.NewRemoteKeySet(ctx, jwksURL), verifierCfg)
	} else {
		provider, err := oidc.NewProvider(ctx, issuer)
		if err != nil {
			return nil, fmt.Errorf("init oidc provider: %w", err)
		}
		verifier = provider.Verifier(verifierCfg)
	}

	return &KeycloakJWKSAuthorizer{
		verifier: &oidcClaimsVerifier{
			verifier: verifier,
		},
		allowMissingJWT: cfg.AllowMissingJWT,
	}, nil
}

func (a *KeycloakJWKSAuthorizer) Authorize(ctx context.Context, _ string, claims *Claims) error {
	if claims == nil {
		return errors.New("claims payload is required")
	}
	rawJWT := strings.TrimSpace(claims.RawJWT)
	if rawJWT == "" {
		if a.allowMissingJWT {
			return nil
		}
		return errors.New("missing bearer token")
	}
	parsed, err := a.verifier.Verify(ctx, rawJWT)
	if err != nil {
		return fmt.Errorf("verify bearer token: %w", err)
	}
	if parsed == nil || strings.TrimSpace(parsed.Subject) == "" {
		return errors.New("token subject is required")
	}

	claims.Subject = strings.TrimSpace(parsed.Subject)
	claims.Email = strings.TrimSpace(parsed.Email)
	claims.EmailVerified = parsed.EmailVerified
	claims.DisplayName = strings.TrimSpace(parsed.DisplayName)
	claims.GivenName = strings.TrimSpace(parsed.GivenName)
	claims.FamilyName = strings.TrimSpace(parsed.FamilyName)
	claims.AvatarURL = strings.TrimSpace(parsed.Picture)
	claims.Locale = strings.TrimSpace(parsed.Locale)
	claims.AuthMethod = strings.TrimSpace(parsed.IdentityProvider)
	claims.Roles = collectUniqueRoles(parsed)
	return nil
}

func collectUniqueRoles(in *tokenClaims) []string {
	if in == nil {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in.RealmAccess.Roles)+len(in.ResourceAccess)*2)

	appendRole := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	for _, role := range in.RealmAccess.Roles {
		appendRole(role)
	}
	for _, set := range in.ResourceAccess {
		for _, role := range set.Roles {
			appendRole(role)
		}
	}
	if len(out) <= 1 {
		return out
	}
	slices.Sort(out)
	return out
}

type oidcClaimsVerifier struct {
	verifier *oidc.IDTokenVerifier
}

func (v *oidcClaimsVerifier) Verify(ctx context.Context, rawJWT string) (*tokenClaims, error) {
	if v == nil || v.verifier == nil {
		return nil, errors.New("oidc verifier is not configured")
	}
	idToken, err := v.verifier.Verify(ctx, rawJWT)
	if err != nil {
		return nil, err
	}
	claims := &tokenClaims{}
	if err := idToken.Claims(claims); err != nil {
		return nil, err
	}
	return claims, nil
}
