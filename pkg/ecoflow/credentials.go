package ecoflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Credentials contains EcoFlow API key material.
type Credentials struct {
	AccessKey string
	SecretKey string
}

// Validate checks that both access and secret keys are present.
func (c Credentials) Validate() error {
	if strings.TrimSpace(c.AccessKey) == "" {
		return errors.New("access key is empty")
	}
	if strings.TrimSpace(c.SecretKey) == "" {
		return errors.New("secret key is empty")
	}
	return nil
}

// CredentialsProvider resolves credentials for a request.
type CredentialsProvider interface {
	Credentials(ctx context.Context) (Credentials, error)
}

// StaticCredentialsProvider always returns the same credentials.
type StaticCredentialsProvider struct {
	credentials Credentials
}

// NewStaticCredentialsProvider creates a static credentials provider.
func NewStaticCredentialsProvider(accessKey, secretKey string) (*StaticCredentialsProvider, error) {
	provider := &StaticCredentialsProvider{
		credentials: Credentials{
			AccessKey: accessKey,
			SecretKey: secretKey,
		},
	}
	if err := provider.credentials.Validate(); err != nil {
		return nil, err
	}
	return provider, nil
}

// Credentials returns the configured static credentials.
func (p *StaticCredentialsProvider) Credentials(_ context.Context) (Credentials, error) {
	return p.credentials, nil
}

// EnvironmentCredentialsProvider loads credentials from environment variables.
type EnvironmentCredentialsProvider struct {
	Environment Environment
	Prefix      string
}

// NewEnvironmentCredentialsProvider builds an env-based provider using the
// default ECOFLOW prefix.
func NewEnvironmentCredentialsProvider(env Environment) *EnvironmentCredentialsProvider {
	return &EnvironmentCredentialsProvider{
		Environment: env,
		Prefix:      "ECOFLOW",
	}
}

// Credentials resolves credentials from environment-specific keys, then
// shared fallback keys.
func (p *EnvironmentCredentialsProvider) Credentials(_ context.Context) (Credentials, error) {
	envUpper := strings.ToUpper(string(p.Environment))
	prefix := strings.TrimSpace(p.Prefix)
	if prefix == "" {
		prefix = "ECOFLOW"
	}

	accessKey := firstNonEmptyEnv(
		fmt.Sprintf("%s_%s_ACCESS_KEY", prefix, envUpper),
		fmt.Sprintf("%s_ACCESS_KEY", prefix),
	)
	secretKey := firstNonEmptyEnv(
		fmt.Sprintf("%s_%s_SECRET_KEY", prefix, envUpper),
		fmt.Sprintf("%s_SECRET_KEY", prefix),
	)

	creds := Credentials{
		AccessKey: accessKey,
		SecretKey: secretKey,
	}
	if err := creds.Validate(); err != nil {
		return Credentials{}, fmt.Errorf(
			"failed loading credentials for env=%s; expected %s_%s_ACCESS_KEY and %s_%s_SECRET_KEY (or shared fallback): %w",
			p.Environment,
			prefix, envUpper,
			prefix, envUpper,
			err,
		)
	}
	return creds, nil
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
