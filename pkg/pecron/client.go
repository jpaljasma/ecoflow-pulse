package pecron

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
	"strconv"
	"strings"
	"time"
)

const (
	appID         = "633"
	appVersion    = "1.9.0"
	appSystemType = "android"
)

type CloudClient interface {
	Login(ctx context.Context, email string, password string) (Session, error)
	ListDevices(ctx context.Context, session Session) ([]Device, error)
	ProductTSL(ctx context.Context, session Session, productKey string) ([]TSLProperty, error)
	DeviceKV(ctx context.Context, session Session, ref DeviceRef) (map[string]any, error)
}

type Client struct {
	region RegionConfig
	http   *http.Client
}

type APIError struct {
	Code    string
	Message string
	Type    string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "pecron api error"
	}
	code := strings.TrimSpace(e.Code)
	if code == "" {
		return message
	}
	return "pecron api code " + code + ": " + message
}

func (e *APIError) RateLimited() bool {
	return e != nil && strings.TrimSpace(e.Code) == "4026"
}

func (e *APIError) DeviceNotBound() bool {
	return e != nil && strings.TrimSpace(e.Code) == "4007"
}

func NewClient(region RegionConfig, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{region: region, http: httpClient}
}

func (c *Client) Login(ctx context.Context, email string, password string) (Session, error) {
	domains := c.region.loginDomains()
	var lastErr error
	for _, domain := range domains {
		session, err := c.loginWithDomain(ctx, email, password, domain)
		if err == nil {
			return session, nil
		}
		if ctx.Err() != nil {
			return Session{}, ctx.Err()
		}
		lastErr = err
		if !pecronDomainRetryable(err) {
			return Session{}, err
		}
	}
	if lastErr != nil {
		return Session{}, lastErr
	}
	return Session{}, errors.New("pecron login domain is not configured")
}

func (c *Client) loginWithDomain(ctx context.Context, email string, password string, domain loginDomain) (Session, error) {
	randomValue, err := GenerateRandom()
	if err != nil {
		return Session{}, err
	}
	encrypted, err := EncryptPassword(password, randomValue)
	if err != nil {
		return Session{}, err
	}
	form := url.Values{}
	form.Set("email", strings.TrimSpace(email))
	form.Set("pwd", encrypted)
	form.Set("random", randomValue)
	form.Set("userDomain", domain.UserDomain)
	form.Set("signature", BuildLoginSignature(strings.TrimSpace(email), encrypted, randomValue, domain.UserDomainSecret))

	var data struct {
		AccessToken struct {
			Token          string `json:"token"`
			ExpirationTime int64  `json:"expirationTime"`
		} `json:"accessToken"`
		RefreshToken struct {
			Token string `json:"token"`
		} `json:"refreshToken"`
		UID string `json:"uid"`
	}
	if err := c.request(ctx, http.MethodPost, "/v2/enduser/enduserapi/emailPwdLogin", "", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", "", &data); err != nil {
		return Session{}, err
	}
	expiresAt := time.Time{}
	if data.AccessToken.ExpirationTime > 0 {
		expiresAt = time.UnixMilli(data.AccessToken.ExpirationTime).UTC()
	}
	claims := decodeJWTClaims(data.AccessToken.Token)
	if data.UID == "" {
		data.UID = claimText(claims, "uid")
	}
	if expiresAt.IsZero() {
		if exp, ok := toFloat(claims["exp"]); ok && exp > 0 {
			expiresAt = time.Unix(int64(exp), 0).UTC()
		}
	}
	return Session{
		AccessToken:  strings.TrimSpace(data.AccessToken.Token),
		RefreshToken: strings.TrimSpace(data.RefreshToken.Token),
		UserID:       strings.TrimSpace(data.UID),
		ExpiresAt:    expiresAt,
	}, nil
}

func (c *Client) RefreshSession(
	ctx context.Context,
	email string,
	password string,
	session Session,
	now time.Time,
	skew time.Duration,
) (Session, error) {
	if !session.NeedsRefresh(now, skew) {
		return session, nil
	}
	return c.Login(ctx, email, password)
}

func (c *Client) ListDevices(ctx context.Context, session Session) ([]Device, error) {
	var raw any
	if err := c.request(ctx, http.MethodGet, "/v2/binding/enduserapi/userDeviceList", "", nil, "", session.AccessToken, &raw); err != nil {
		return nil, err
	}
	items := rawList(raw)
	out := make([]Device, 0, len(items))
	for _, item := range items {
		record := asMap(item)
		ref := DeviceRef{
			ProductKey: strings.TrimSpace(asString(record["productKey"])),
			DeviceKey:  strings.TrimSpace(asString(record["deviceKey"])),
		}
		if ref.ProductKey == "" || ref.DeviceKey == "" {
			continue
		}
		out = append(out, Device{
			DeviceName:     firstText(record, "deviceName", "name"),
			ProductKey:     ref.ProductKey,
			DeviceKey:      ref.DeviceKey,
			ProductName:    firstText(record, "productName"),
			Online:         asFloat(record["onlineStatus"]) == 1,
			Protocol:       firstText(record, "protocol"),
			DeviceSN:       firstText(record, "sn"),
			SignalStrength: int(asFloat(record["signalStrength"])),
			LastConnTime:   firstText(record, "lastConnTime"),
		})
	}
	return out, nil
}

func (c *Client) ProductTSL(ctx context.Context, session Session, productKey string) ([]TSLProperty, error) {
	productKey = strings.TrimSpace(productKey)
	if productKey == "" {
		return nil, errors.New("product key is required")
	}
	query := "pk=" + url.QueryEscape(productKey)
	var raw any
	if err := c.request(ctx, http.MethodGet, "/v2/binding/enduserapi/productTSL", query, nil, "", session.AccessToken, &raw); err != nil {
		return nil, err
	}
	props := productTSLProperties(raw)
	out := make([]TSLProperty, 0, len(props))
	for _, item := range props {
		record := asMap(item)
		code := firstText(record, "code", "identifier", "resourceCode")
		if code == "" {
			continue
		}
		accessMode := firstText(record, "accessMode", "subType", "mode")
		dataType := describeTSLDataType(record["dataType"])
		out = append(out, TSLProperty{
			Code:       code,
			Name:       firstText(record, "name", "desc", "description"),
			DataType:   dataType,
			AccessMode: accessMode,
			Writable:   tslAccessWritable(accessMode),
			Raw:        cloneMap(record),
		})
	}
	return out, nil
}

func (c *Client) DeviceKV(ctx context.Context, session Session, ref DeviceRef) (map[string]any, error) {
	query := "pk=" + url.QueryEscape(ref.ProductKey) + "&dk=" + url.QueryEscape(ref.DeviceKey)
	var raw struct {
		CustomizeTSLInfo []struct {
			ResourceCode        string `json:"resourceCode"`
			ResourceValue       any    `json:"resourceValce"`
			ResourceValueAlias  any    `json:"resourceValue"`
			ResourceValueAlias2 any    `json:"value"`
			DataType            any    `json:"dataType"`
		} `json:"customizeTslInfo"`
	}
	if err := c.request(ctx, http.MethodGet, "/v2/binding/enduserapi/getDeviceBusinessAttributes", query, nil, "", session.AccessToken, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]any, len(raw.CustomizeTSLInfo))
	for _, row := range raw.CustomizeTSLInfo {
		code := strings.TrimSpace(row.ResourceCode)
		rawValue := firstNonEmptyPresent(row.ResourceValue, row.ResourceValueAlias, row.ResourceValueAlias2)
		if code == "" || rawValue == nil {
			continue
		}
		value := convertRESTValue(rawValue, row.DataType)
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			continue
		}
		out[code] = value
	}
	return out, nil
}

func (c *Client) request(
	ctx context.Context,
	method string,
	path string,
	rawQuery string,
	body io.Reader,
	contentType string,
	token string,
	out any,
) error {
	if c == nil {
		return fmt.Errorf("pecron client is required")
	}
	base := strings.TrimRight(c.region.BaseURL, "/")
	endpoint := base + path
	if rawQuery != "" {
		endpoint += "?" + rawQuery
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("build pecron request: %w", err)
	}
	req.Header.Set("X-Q-Language", "en")
	req.Header.Set("appId", appID)
	req.Header.Set("appVersion", appVersion)
	req.Header.Set("appSystemType", appSystemType)
	req.Header.Set("app-info", "[Pulse][Go][pecron-cloud][1]")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", strings.TrimSpace(token))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send pecron request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("read pecron response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pecron request status %d", resp.StatusCode)
	}
	var envelope struct {
		Code any             `json:"code"`
		Msg  string          `json:"msg"`
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("decode pecron response: %w", err)
	}
	if codeString(envelope.Code) != "200" {
		return &APIError{
			Code:    codeString(envelope.Code),
			Message: envelope.Msg,
			Type:    envelope.Type,
		}
	}
	if out == nil {
		return nil
	}
	if len(envelope.Data) == 0 || bytes.Equal(envelope.Data, []byte("null")) {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode pecron data: %w", err)
	}
	return nil
}

func pecronDomainRetryable(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch strings.TrimSpace(apiErr.Code) {
	case "5015", "5031", "5420":
		return true
	default:
		return false
	}
}

func rawList(raw any) []any {
	switch value := raw.(type) {
	case []any:
		return value
	case map[string]any:
		if list, ok := value["list"].([]any); ok {
			return list
		}
	}
	return nil
}

func productTSLProperties(raw any) []any {
	if list := rawList(raw); len(list) > 0 {
		return list
	}
	record := asMap(raw)
	if len(record) == 0 {
		return nil
	}
	if props := rawList(record["properties"]); len(props) > 0 {
		return props
	}
	tslJSON := strings.TrimSpace(asString(record["tslJson"]))
	if tslJSON == "" {
		return nil
	}
	var decoded any
	if err := json.Unmarshal([]byte(tslJSON), &decoded); err != nil {
		return nil
	}
	decodedRecord := asMap(decoded)
	if len(decodedRecord) == 0 {
		return nil
	}
	return rawList(decodedRecord["properties"])
}

func firstText(record map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(asString(record[key])); value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func convertRESTValue(value any, dataType any) any {
	if nested := asMap(value); len(nested) > 0 {
		return nested
	}
	raw := strings.TrimSpace(asString(value))
	if raw == "" || raw == "<nil>" {
		return ""
	}
	kind := strings.ToLower(strings.TrimSpace(describeTSLDataType(dataType)))
	if strings.Contains(kind, "struct") || strings.Contains(kind, "object") || strings.Contains(kind, "json") {
		var parsed any
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
			return parsed
		}
	}
	if strings.Contains(kind, "bool") {
		if b, ok := toBool(raw); ok {
			return b
		}
	}
	if strings.Contains(kind, "int") || strings.Contains(kind, "long") || strings.Contains(kind, "enum") || strings.Contains(kind, "number") {
		if f, ok := toFloat(raw); ok {
			return f
		}
	}
	if f, ok := toFloat(raw); ok {
		return f
	}
	return raw
}

func decodeJWTClaims(token string) map[string]any {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) < 2 {
		return nil
	}
	payload := parts[1]
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(payload + strings.Repeat("=", (4-len(payload)%4)%4))
		if err != nil {
			return nil
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil
	}
	return claims
}

func claimText(claims map[string]any, key string) string {
	if len(claims) == 0 {
		return ""
	}
	value := strings.TrimSpace(asString(claims[key]))
	if value == "<nil>" {
		return ""
	}
	return value
}

func firstNonEmptyPresent(values ...any) any {
	for _, value := range values {
		if value == nil {
			continue
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			continue
		}
		return value
	}
	return nil
}

func describeTSLDataType(value any) string {
	record := asMap(value)
	if len(record) > 0 {
		return firstText(record, "type", "name")
	}
	if text := strings.TrimSpace(asString(value)); text != "" && text != "<nil>" {
		return text
	}
	return ""
}

func tslAccessWritable(accessMode string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(accessMode))
	return strings.Contains(normalized, "W")
}

func codeString(code any) string {
	switch value := code.(type) {
	case float64:
		return strconv.FormatInt(int64(value), 10)
	case int:
		return strconv.Itoa(value)
	case string:
		return strings.TrimSpace(value)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}
