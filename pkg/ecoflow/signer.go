package ecoflow

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const (
	// HeaderAccessKey is the request header containing EcoFlow access key.
	HeaderAccessKey = "accessKey"
	// HeaderNonce is the request header containing per-request nonce.
	HeaderNonce = "nonce"
	// HeaderTimestamp is the request header containing unix milliseconds.
	HeaderTimestamp = "timestamp"
	// HeaderSignature is the request header containing HMAC signature.
	HeaderSignature = "sign"
)

// SignInput is the canonicalized payload used for request signing.
type SignInput struct {
	Credentials     Credentials
	Nonce           string
	TimestampMillis int64
	Query           url.Values
	Body            any
}

// SignResult contains the generated signature and canonical string.
type SignResult struct {
	Signature string
	Canonical string
}

// Signer signs EcoFlow requests.
type Signer interface {
	Sign(ctx context.Context, in SignInput) (SignResult, error)
}

// HMACSHA256Signer signs requests using HMAC-SHA256.
type HMACSHA256Signer struct{}

// NewHMACSHA256Signer returns a Signer implementing EcoFlow HMAC-SHA256 signing.
func NewHMACSHA256Signer() *HMACSHA256Signer {
	return &HMACSHA256Signer{}
}

// Sign signs request data and returns signature metadata.
func (s *HMACSHA256Signer) Sign(_ context.Context, in SignInput) (SignResult, error) {
	if err := in.Credentials.Validate(); err != nil {
		return SignResult{}, err
	}
	if strings.TrimSpace(in.Nonce) == "" {
		return SignResult{}, fmt.Errorf("nonce is required")
	}
	if in.TimestampMillis <= 0 {
		return SignResult{}, fmt.Errorf("timestamp must be > 0")
	}

	canonical, err := buildCanonicalString(in)
	if err != nil {
		return SignResult{}, err
	}

	mac := hmac.New(sha256.New, []byte(in.Credentials.SecretKey))
	if _, err := mac.Write([]byte(canonical)); err != nil {
		return SignResult{}, fmt.Errorf("write hmac payload: %w", err)
	}

	return SignResult{
		Signature: hex.EncodeToString(mac.Sum(nil)),
		Canonical: canonical,
	}, nil
}

func buildCanonicalString(in SignInput) (string, error) {
	systemParams := []kv{
		kv{key: HeaderAccessKey, value: in.Credentials.AccessKey},
		kv{key: HeaderNonce, value: in.Nonce},
		kv{key: HeaderTimestamp, value: strconv.FormatInt(in.TimestampMillis, 10)},
	}
	sort.Slice(systemParams, func(i, j int) bool {
		return systemParams[i].key < systemParams[j].key
	})

	queryParams := make([]kv, 0, len(in.Query))
	appendQueryParams(&queryParams, in.Query)

	bodyParams, err := canonicalBodyParams(in.Body)
	if err != nil {
		return "", err
	}
	requestParams := make([]kv, 0, len(queryParams)+len(bodyParams))
	requestParams = append(requestParams, queryParams...)
	requestParams = append(requestParams, bodyParams...)
	sort.Slice(requestParams, func(i, j int) bool {
		if requestParams[i].key == requestParams[j].key {
			return requestParams[i].value < requestParams[j].value
		}
		return requestParams[i].key < requestParams[j].key
	})
	params := make([]kv, 0, len(systemParams)+len(requestParams))
	params = append(params, requestParams...)
	params = append(params, systemParams...)

	var b strings.Builder
	for i, p := range params {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(p.key)
		b.WriteByte('=')
		b.WriteString(p.value)
	}
	return b.String(), nil
}

type kv struct {
	key   string
	value string
}

func appendQueryParams(dest *[]kv, query url.Values) {
	if len(query) == 0 {
		return
	}

	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		values := append([]string(nil), query[key]...)
		sort.Strings(values)
		for _, value := range values {
			*dest = append(*dest, kv{key: key, value: value})
		}
	}
}

func canonicalBodyParams(body any) ([]kv, error) {
	if body == nil {
		return nil, nil
	}

	if asMap, ok := body.(map[string]any); ok {
		return mapToKVs(asMap), nil
	}
	if asMap, ok := body.(map[string]string); ok {
		out := make(map[string]any, len(asMap))
		for key, value := range asMap {
			out[key] = value
		}
		return mapToKVs(out), nil
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body for signing: %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal(encoded, &m); err != nil {
		// For non-object JSON payloads, sign a stable compact JSON blob as "body".
		return []kv{{key: "body", value: string(encoded)}}, nil
	}

	return mapToKVs(m), nil
}

func mapToKVs(input map[string]any) []kv {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]kv, 0, len(input))
	for _, key := range keys {
		out = append(out, kv{
			key:   key,
			value: stringifyValue(input[key]),
		})
	}
	return out
}

func stringifyValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case json.Number:
		return v.String()
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(encoded)
	}
}
