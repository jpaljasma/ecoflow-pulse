package ecoflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const deviceListPath = "/iot-open/sign/device/list"
const mqttCertificationPath = "/iot-open/sign/certification"
const deviceQuotaAllPath = "/iot-open/sign/device/quota/all"

// GeneralInfoService contains generic signed request helpers from the EcoFlow
// "general info" authentication model.
type GeneralInfoService struct {
	client *Client
}

// GeneralInfoDevice describes one EcoFlow device entry returned by the
// general-info device list API.
type GeneralInfoDevice struct {
	// SN is the device serial number.
	SN string `json:"sn"`
	// DeviceName is the user-visible device name.
	DeviceName string `json:"deviceName"`
	// ProductName is the product model name when provided by EcoFlow.
	ProductName string `json:"productName"`
	// Online indicates connectivity state: 0 means offline, 1 means online.
	Online int `json:"online"`
}

// GeneralInfoDevicesResponse is the typed payload for device list responses.
type GeneralInfoDevicesResponse struct {
	Code    string              `json:"code"`
	Message string              `json:"message"`
	Data    []GeneralInfoDevice `json:"data"`
}

// GeneralInfoMQTTCertification describes MQTT credentials and endpoint details
// returned by EcoFlow for authenticated MQTT communication.
type GeneralInfoMQTTCertification struct {
	// CertificateAccount is the MQTT username/account.
	CertificateAccount string `json:"certificateAccount"`
	// CertificatePassword is the MQTT password/token.
	CertificatePassword string `json:"certificatePassword"`
	// URL is the MQTT broker hostname.
	URL string `json:"url"`
	// Port is the MQTT broker port represented by the API as a string.
	Port string `json:"port"`
	// Protocol is the transport protocol, for example "mqtts".
	Protocol string `json:"protocol"`
}

// GeneralInfoMQTTCertificationResponse is the typed envelope returned by
// the MQTT certification endpoint.
type GeneralInfoMQTTCertificationResponse struct {
	Code    string                       `json:"code"`
	Message string                       `json:"message"`
	Data    GeneralInfoMQTTCertification `json:"data"`
}

// GeneralInfoDeviceAllQuotaResponse is the typed envelope for the
// device all-quota endpoint.
type GeneralInfoDeviceAllQuotaResponse struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

// GeneralInfo returns a helper for issuing signed generic requests.
func (c *Client) GeneralInfo() *GeneralInfoService {
	return &GeneralInfoService{client: c}
}

// SignedRequest performs a signed request with explicit method/path/query/body.
func (s *GeneralInfoService) SignedRequest(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	body any,
) (Response, error) {
	return s.client.Do(ctx, Request{
		Method: method,
		Path:   path,
		Query:  query,
		Body:   body,
	})
}

// SignedGet performs a signed HTTP GET request.
func (s *GeneralInfoService) SignedGet(ctx context.Context, path string, query url.Values) (Response, error) {
	return s.client.Do(ctx, Request{
		Method: http.MethodGet,
		Path:   path,
		Query:  query,
	})
}

// ListDevices loads the signed EcoFlow device list and decodes it into typed
// device objects.
func (s *GeneralInfoService) ListDevices(ctx context.Context) ([]GeneralInfoDevice, Response, error) {
	var payload GeneralInfoDevicesResponse
	response, err := s.client.DoJSON(ctx, Request{
		Method: http.MethodGet,
		Path:   deviceListPath,
	}, &payload)
	if err != nil {
		return nil, response, err
	}
	if err := validateGeneralInfoCode(payload.Code, payload.Message); err != nil {
		return nil, response, err
	}
	return payload.Data, response, nil
}

// GetMQTTCertification fetches MQTT connection credentials and endpoint data
// for EcoFlow device messaging.
func (s *GeneralInfoService) GetMQTTCertification(
	ctx context.Context,
) (GeneralInfoMQTTCertification, Response, error) {
	var payload GeneralInfoMQTTCertificationResponse
	response, err := s.client.DoJSON(ctx, Request{
		Method: http.MethodGet,
		Path:   mqttCertificationPath,
	}, &payload)
	if err != nil {
		return GeneralInfoMQTTCertification{}, response, err
	}
	if err := validateGeneralInfoCode(payload.Code, payload.Message); err != nil {
		return GeneralInfoMQTTCertification{}, response, err
	}
	return payload.Data, response, nil
}

// GetDeviceAllQuota fetches all available quota values for one device serial
// number and returns them as key-value pairs.
func (s *GeneralInfoService) GetDeviceAllQuota(
	ctx context.Context,
	sn string,
) (map[string]string, Response, error) {
	sn = strings.TrimSpace(sn)
	if sn == "" {
		return nil, Response{}, fmt.Errorf("sn is required")
	}

	var payload GeneralInfoDeviceAllQuotaResponse
	response, err := s.client.DoJSON(ctx, Request{
		Method: http.MethodGet,
		Path:   deviceQuotaAllPath,
		Query: url.Values{
			"sn": []string{sn},
		},
	}, &payload)
	if err != nil {
		return nil, response, err
	}
	if err := validateGeneralInfoCode(payload.Code, payload.Message); err != nil {
		return nil, response, err
	}
	normalized, err := normalizeQuotaValues(payload.Data)
	if err != nil {
		return nil, response, err
	}
	return normalized, response, nil
}

func validateGeneralInfoCode(code, message string) error {
	if code == "" || code == "0" {
		return nil
	}
	return &BusinessError{
		Code:    code,
		Message: message,
	}
}

func normalizeQuotaValues(input map[string]any) (map[string]string, error) {
	if len(input) == 0 {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		switch v := value.(type) {
		case nil:
			out[key] = ""
		case json.Number:
			out[key] = v.String()
		default:
			out[key] = stringifyValue(v)
		}
	}
	return out, nil
}
