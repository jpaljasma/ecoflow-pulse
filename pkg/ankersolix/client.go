package ankersolix

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	endpointLogin       = "passport/login"
	endpointBindDevices = "power_service/v1/app/get_relate_and_bind_devices"
	endpointMQTTInfo    = "app/devicemanage/get_user_mqtt_info"
)

type CloudClient interface {
	Login(ctx context.Context, email string, password string) (Session, error)
	ListBindDevices(ctx context.Context, session Session) ([]Device, error)
	GetMQTTInfo(ctx context.Context, session Session) (MQTTSessionInfo, error)
}

type Client struct {
	cfg     Config
	http    *http.Client
	now     func() time.Time
	encrypt func(password string) (EncryptedPassword, error)
}

func NewClient(cfg Config, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	cfg = normalizeConfig(cfg)
	return &Client{
		cfg:  cfg,
		http: httpClient,
		now:  func() time.Time { return time.Now() },
		encrypt: func(password string) (EncryptedPassword, error) {
			return EncryptPassword(PasswordEncryptionInput{Password: password})
		},
	}
}

func (c *Client) Login(ctx context.Context, email string, password string) (Session, error) {
	if c == nil {
		return Session{}, errors.New("anker solix client is required")
	}
	encrypted, err := c.encrypt(password)
	if err != nil {
		return Session{}, err
	}
	now := c.now().Local()
	offsetSeconds := 0
	if _, offset := now.Zone(); offset != 0 {
		offsetSeconds = offset
	}
	body := map[string]any{
		"ab": c.cfg.Country,
		"client_secret_info": map[string]any{
			"public_key": encrypted.ClientPublicKeyHex,
		},
		"enc":         0,
		"email":       strings.TrimSpace(email),
		"password":    encrypted.CiphertextBase64,
		"time_zone":   offsetSeconds * 1000,
		"transaction": fmt.Sprintf("%d", now.UnixMilli()),
	}
	var data struct {
		UserID         string `json:"user_id"`
		AuthToken      string `json:"auth_token"`
		TokenExpiresAt int64  `json:"token_expires_at"`
		Nickname       string `json:"nick_name"`
	}
	if err := c.request(ctx, http.MethodPost, endpointLogin, Session{}, body, &data); err != nil {
		return Session{}, err
	}
	return Session{
		UserID:         strings.TrimSpace(data.UserID),
		AuthToken:      strings.TrimSpace(data.AuthToken),
		TokenExpiresAt: data.TokenExpiresAt,
		Nickname:       strings.TrimSpace(data.Nickname),
	}, nil
}

func (c *Client) ListBindDevices(ctx context.Context, session Session) ([]Device, error) {
	var raw any
	if err := c.request(ctx, http.MethodPost, endpointBindDevices, session, map[string]any{}, &raw); err != nil {
		return nil, err
	}
	items := rawList(raw)
	out := make([]Device, 0, len(items))
	for _, item := range items {
		record := asMap(item)
		ref := DeviceRef{
			ProductCode: firstText(record, "product_code", "device_pn", "device_model", "pn", "productCode"),
			DeviceSN:    firstText(record, "device_sn", "sn", "deviceSn"),
		}.Normalize()
		if ref.ProductCode == "" || ref.DeviceSN == "" {
			continue
		}
		name := firstText(record, "device_name", "name", "product_name")
		alias := firstText(record, "alias_name", "alias", "nickname")
		out = append(out, Device{
			DeviceSN:    ref.DeviceSN,
			ProductCode: ref.ProductCode,
			Name:        name,
			Alias:       alias,
			DeviceName:  name,
			AliasName:   alias,
			Firmware:    firstText(record, "firmware", "fw_version", "version"),
			Online:      recordBool(record, "wifi_online", "online", "is_online"),
			OwnerUserID: firstText(record, "owner_user_id", "user_id"),
			SiteID:      firstText(record, "site_id", "siteId"),
			Raw:         cloneMap(record),
		})
	}
	return out, nil
}

func (c *Client) GetMQTTInfo(ctx context.Context, session Session) (MQTTSessionInfo, error) {
	var info MQTTSessionInfo
	if err := c.request(ctx, http.MethodPost, endpointMQTTInfo, session, map[string]any{}, &info); err != nil {
		return MQTTSessionInfo{}, err
	}
	info.Normalize()
	return info, nil
}

func (c *Client) request(ctx context.Context, method string, path string, session Session, body any, out any) error {
	if c == nil {
		return errors.New("anker solix client is required")
	}
	if err := c.cfg.Validate(); err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode anker solix request: %w", err)
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL()+"/"+strings.TrimLeft(path, "/"), reader)
	if err != nil {
		return fmt.Errorf("build anker solix request: %w", err)
	}
	for key, values := range TokenHeaders(c.cfg, session, timezoneString(c.now())) {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send anker solix request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("read anker solix response: %w", err)
	}
	var envelope apiEnvelope
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &envelope)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
			HTTPStatus: resp.StatusCode,
			Code:       codeInt(envelope.Code, resp.StatusCode),
			Message:    firstNonEmpty(envelope.Message, resp.Status),
		}
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("decode anker solix response: %w", err)
	}
	code := codeInt(envelope.Code, 0)
	if code != 0 && code != 200 {
		return &APIError{HTTPStatus: resp.StatusCode, Code: code, Message: envelope.Message}
	}
	if out == nil || len(envelope.Data) == 0 || bytes.Equal(envelope.Data, []byte("null")) {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode anker solix data: %w", err)
	}
	return nil
}

type apiEnvelope struct {
	Code    any             `json:"code"`
	Message string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

type APIError struct {
	HTTPStatus int
	Code       int
	Message    string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "anker solix api error"
	}
	if e.Code != 0 {
		return fmt.Sprintf("anker solix api code %d: %s", e.Code, message)
	}
	if e.HTTPStatus != 0 {
		return fmt.Sprintf("anker solix api status %d: %s", e.HTTPStatus, message)
	}
	return message
}

func (e *APIError) Retryable() bool {
	if e == nil {
		return false
	}
	if e.HTTPStatus == http.StatusTooManyRequests || e.HTTPStatus >= 500 {
		return true
	}
	switch e.Code {
	case 429, 997, 998, 999, 21105, 100053:
		return true
	default:
		return false
	}
}

func IsRetryable(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Retryable()
}

func recordBool(record map[string]any, keys ...string) bool {
	for _, key := range keys {
		value, ok := record[key]
		if !ok {
			continue
		}
		if b, ok := toBool(value); ok {
			return b
		}
	}
	return false
}

func codeInt(value any, fallback int) int {
	if f, ok := toFloat(value); ok {
		return int(f)
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func timezoneString(now time.Time) string {
	now = now.Local()
	_, offset := now.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return fmt.Sprintf("GMT%s%02d:%02d", sign, offset/3600, offset%3600/60)
}
