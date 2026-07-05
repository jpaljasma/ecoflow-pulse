package ecoflow

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultAppLoginPath = "/auth/login"
)

// AppLoginSession contains the temporary result of an EcoFlow app login.
// Token is intentionally returned only to the caller and should not be stored
// when deriving BLE auth material.
type AppLoginSession struct {
	UserID string
	Name   string
	Token  string
}

// AppLoginClient exchanges EcoFlow account credentials against the app login
// surface so Pulse can derive account-scoped BLE auth material.
type AppLoginClient struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
}

// LoginApp exchanges a one-time EcoFlow email/password for the app user ID.
func (c AppLoginClient) LoginApp(ctx context.Context, email string, password string) (AppLoginSession, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return AppLoginSession{}, errors.New("email required")
	}
	if strings.TrimSpace(password) == "" {
		return AppLoginSession{}, errors.New("password required")
	}

	endpoint, err := c.loginURL()
	if err != nil {
		return AppLoginSession{}, err
	}
	body, err := json.Marshal(map[string]string{
		"email":    email,
		"password": base64.StdEncoding.EncodeToString([]byte(password)),
		"scene":    "IOT_APP",
		"userType": "ECOFLOW",
	})
	if err != nil {
		return AppLoginSession{}, fmt.Errorf("marshal app login request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return AppLoginSession{}, fmt.Errorf("build app login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if ua := strings.TrimSpace(c.UserAgent); ua != "" {
		req.Header.Set("User-Agent", ua)
	}

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return AppLoginSession{}, fmt.Errorf("ecoflow app login request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return AppLoginSession{}, fmt.Errorf("read ecoflow app login response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return AppLoginSession{}, &HTTPError{StatusCode: resp.StatusCode, Body: respBody}
	}

	var decoded appLoginResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return AppLoginSession{}, fmt.Errorf("decode ecoflow app login response: %w", err)
	}
	if err := validateBusinessCode(normalizedBusinessCode(decoded.Code), decoded.Message); err != nil {
		return AppLoginSession{}, err
	}
	userID := strings.TrimSpace(decoded.Data.User.UserID)
	if userID == "" {
		return AppLoginSession{}, errors.New("ecoflow app login response missing user id")
	}
	return AppLoginSession{
		UserID: userID,
		Name:   strings.TrimSpace(decoded.Data.User.Name),
		Token:  strings.TrimSpace(decoded.Data.Token),
	}, nil
}

func (c AppLoginClient) loginURL() (*url.URL, error) {
	base := strings.TrimSpace(c.BaseURL)
	if base == "" {
		base = defaultBaseURL
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse ecoflow app login base url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, errors.New("ecoflow app login base url must include scheme and host")
	}
	u.Path = strings.TrimRight(u.Path, "/") + defaultAppLoginPath
	u.RawQuery = ""
	u.Fragment = ""
	return u, nil
}

type appLoginResponse struct {
	Code    any    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Token string `json:"token"`
		User  struct {
			UserID string `json:"userId"`
			Name   string `json:"name"`
		} `json:"user"`
	} `json:"data"`
}

func normalizedBusinessCode(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		return fmt.Sprintf("%.0f", v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
