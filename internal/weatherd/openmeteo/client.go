package openmeteo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/cachekey"
)

const (
	DefaultForecastBaseURL           = "https://api.open-meteo.com/v1/forecast"
	DefaultHistoricalForecastBaseURL = "https://historical-forecast-api.open-meteo.com/v1/forecast"
)

type Client struct {
	httpClient                *http.Client
	forecastBaseURL           string
	historicalForecastBaseURL string
}

type Config struct {
	HTTPClient                *http.Client
	ForecastBaseURL           string
	HistoricalForecastBaseURL string
}

func NewClient(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	forecastBaseURL := strings.TrimSpace(cfg.ForecastBaseURL)
	if forecastBaseURL == "" {
		forecastBaseURL = DefaultForecastBaseURL
	}
	historicalForecastBaseURL := strings.TrimSpace(cfg.HistoricalForecastBaseURL)
	if historicalForecastBaseURL == "" {
		historicalForecastBaseURL = DefaultHistoricalForecastBaseURL
	}
	return &Client{
		httpClient:                httpClient,
		forecastBaseURL:           forecastBaseURL,
		historicalForecastBaseURL: historicalForecastBaseURL,
	}
}

func (c *Client) FetchForecast(ctx context.Context, req weatherd.Request) (*weatherd.Bundle, error) {
	bundles, err := c.fetch(ctx, c.forecastBaseURL, []weatherd.Request{req})
	if err != nil {
		return nil, err
	}
	if len(bundles) == 0 {
		return nil, fmt.Errorf("open-meteo forecast returned no bundles")
	}
	return &bundles[0], nil
}

func (c *Client) FetchForecastBatch(ctx context.Context, reqs []weatherd.Request) ([]weatherd.Bundle, error) {
	return c.fetch(ctx, c.forecastBaseURL, reqs)
}

func (c *Client) FetchHistoricalForecast(ctx context.Context, req weatherd.Request) (*weatherd.Bundle, error) {
	bundles, err := c.fetch(ctx, c.historicalForecastBaseURL, []weatherd.Request{req})
	if err != nil {
		return nil, err
	}
	if len(bundles) == 0 {
		return nil, fmt.Errorf("open-meteo historical forecast returned no bundles")
	}
	return &bundles[0], nil
}

func (c *Client) fetch(ctx context.Context, baseURL string, reqs []weatherd.Request) ([]weatherd.Bundle, error) {
	if len(reqs) == 0 {
		return nil, nil
	}
	normalized := make([]weatherd.Request, 0, len(reqs))
	for _, req := range reqs {
		normalized = append(normalized, req.Normalized())
	}
	endpoint, err := buildURL(baseURL, normalized)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create open-meteo request: %w", err)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call open-meteo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read open-meteo body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("open-meteo status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return decodeBundles(body)
}

func buildURL(baseURL string, reqs []weatherd.Request) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse open-meteo base url: %w", err)
	}
	latitudes := make([]string, 0, len(reqs))
	longitudes := make([]string, 0, len(reqs))
	for _, req := range reqs {
		latitudes = append(latitudes, formatFloat(req.Latitude))
		longitudes = append(longitudes, formatFloat(req.Longitude))
	}
	values := parsed.Query()
	values.Set("latitude", strings.Join(latitudes, ","))
	values.Set("longitude", strings.Join(longitudes, ","))
	values.Set("forecast_days", "7")
	values.Set("past_days", "1")
	values.Set("timezone", normalizedTimezone(reqs[0].Timezone))
	values.Set("temperature_unit", "celsius")
	values.Set("wind_speed_unit", "kmh")
	values.Set("precipitation_unit", "mm")
	values.Set("hourly", strings.Join(hourlyFields(reqs[0].PanelTiltDegrees), ","))
	values.Set("daily", strings.Join(dailyFields(), ","))
	if reqs[0].PanelTiltDegrees != nil {
		values.Set("tilt", formatFloat(*cachekey.TiltBucket(reqs[0].PanelTiltDegrees)))
		azimuth := reqs[0].PanelAzimuthDegrees
		if azimuth != nil {
			values.Set("azimuth", formatFloat(*cachekey.AzimuthBucket(azimuth)))
		}
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func normalizedTimezone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "auto"
	}
	return value
}

func hourlyFields(tilt *float64) []string {
	fields := []string{
		"weather_code",
		"temperature_2m",
		"wind_speed_10m",
		"wind_direction_10m",
		"precipitation",
		"cloud_cover",
		"visibility",
		"sunshine_duration",
		"shortwave_radiation",
		"uv_index",
	}
	if tilt != nil {
		fields = append(fields, "global_tilted_irradiance")
	}
	return fields
}

func dailyFields() []string {
	return []string{
		"weather_code",
		"sunrise",
		"sunset",
		"daylight_duration",
		"sunshine_duration",
		"shortwave_radiation_sum",
		"uv_index_max",
	}
}

type openMeteoEnvelope struct {
	Latitude             float64         `json:"latitude"`
	Longitude            float64         `json:"longitude"`
	Elevation            float64         `json:"elevation"`
	Timezone             string          `json:"timezone"`
	TimezoneAbbreviation string          `json:"timezone_abbreviation"`
	Hourly               openMeteoHourly `json:"hourly"`
	Daily                openMeteoDaily  `json:"daily"`
}

type openMeteoHourly struct {
	Time                   []string   `json:"time"`
	WeatherCode            []int32    `json:"weather_code"`
	Temperature2M          []float64  `json:"temperature_2m"`
	WindSpeed10M           []float64  `json:"wind_speed_10m"`
	WindDirection10M       []float64  `json:"wind_direction_10m"`
	Precipitation          []float64  `json:"precipitation"`
	CloudCover             []float64  `json:"cloud_cover"`
	Visibility             []float64  `json:"visibility"`
	SunshineDuration       []float64  `json:"sunshine_duration"`
	ShortwaveRadiation     []float64  `json:"shortwave_radiation"`
	UVIndex                []*float64 `json:"uv_index"`
	GlobalTiltedIrradiance []*float64 `json:"global_tilted_irradiance"`
}

type openMeteoDaily struct {
	Time                  []string   `json:"time"`
	WeatherCode           []int32    `json:"weather_code"`
	Sunrise               []string   `json:"sunrise"`
	Sunset                []string   `json:"sunset"`
	DaylightDuration      []float64  `json:"daylight_duration"`
	SunshineDuration      []float64  `json:"sunshine_duration"`
	ShortwaveRadiationSum []float64  `json:"shortwave_radiation_sum"`
	UVIndexMax            []*float64 `json:"uv_index_max"`
}

func decodeBundles(body []byte) ([]weatherd.Bundle, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("open-meteo returned empty body")
	}
	if trimmed[0] == '[' {
		var rows []openMeteoEnvelope
		if err := json.Unmarshal(trimmed, &rows); err != nil {
			return nil, fmt.Errorf("decode open-meteo batch response: %w", err)
		}
		out := make([]weatherd.Bundle, 0, len(rows))
		for _, row := range rows {
			bundle, err := envelopeToBundle(row)
			if err != nil {
				return nil, err
			}
			out = append(out, bundle)
		}
		return out, nil
	}
	var row openMeteoEnvelope
	if err := json.Unmarshal(trimmed, &row); err != nil {
		return nil, fmt.Errorf("decode open-meteo response: %w", err)
	}
	bundle, err := envelopeToBundle(row)
	if err != nil {
		return nil, err
	}
	return []weatherd.Bundle{bundle}, nil
}

func envelopeToBundle(in openMeteoEnvelope) (weatherd.Bundle, error) {
	loc, err := timeLocation(in.Timezone)
	if err != nil {
		return weatherd.Bundle{}, err
	}
	provenance := weatherd.Provenance{
		Source:               "open_meteo",
		ModelSelection:       "best_match",
		ActualSource:         "past_days",
		Timezone:             in.Timezone,
		TimezoneAbbreviation: in.TimezoneAbbreviation,
		IssuedAt:             time.Now().UTC(),
		Latitude:             in.Latitude,
		Longitude:            in.Longitude,
		Elevation:            in.Elevation,
	}
	provenance.CanonicalLocationKey = cachekey.Build(cachekey.CanonicalLocation{
		Latitude:  in.Latitude,
		Longitude: in.Longitude,
		Elevation: in.Elevation,
	})
	out := weatherd.Bundle{
		Provenance: provenance,
		Hourly:     make([]weatherd.HourlyForecastPoint, 0, len(in.Hourly.Time)),
		Daily:      make([]weatherd.DailyForecastPoint, 0, len(in.Daily.Time)),
	}
	for idx, rawTime := range in.Hourly.Time {
		t, err := time.ParseInLocation("2006-01-02T15:04", rawTime, loc)
		if err != nil {
			return weatherd.Bundle{}, fmt.Errorf("parse open-meteo hourly time %q: %w", rawTime, err)
		}
		point := weatherd.HourlyForecastPoint{
			Time: t.UTC(),
			Condition: weatherd.WeatherCondition{
				WeatherCode: valueAtInt32(in.Hourly.WeatherCode, idx),
			},
			Raw: weatherd.ForecastValueSet{
				Temperature:             valueAtFloat64(in.Hourly.Temperature2M, idx),
				WindSpeed:               valueAtFloat64(in.Hourly.WindSpeed10M, idx),
				WindDirectionDegrees:    valueAtFloat64(in.Hourly.WindDirection10M, idx),
				Precipitation:           valueAtFloat64(in.Hourly.Precipitation, idx),
				CloudCover:              valueAtFloat64(in.Hourly.CloudCover, idx),
				Visibility:              valueAtFloat64(in.Hourly.Visibility, idx),
				SunshineDurationSeconds: valueAtFloat64(in.Hourly.SunshineDuration, idx),
				ShortwaveRadiation:      valueAtFloat64(in.Hourly.ShortwaveRadiation, idx),
				UVIndex:                 pointerAtFloat64(in.Hourly.UVIndex, idx),
				GlobalTiltedIrradiance:  pointerAtFloat64(in.Hourly.GlobalTiltedIrradiance, idx),
			},
		}
		point.Condition.WeatherText = WeatherCodeText(point.Condition.WeatherCode)
		point.Corrected = point.Raw
		out.Hourly = append(out.Hourly, point)
	}
	for idx, rawDate := range in.Daily.Time {
		date, err := time.ParseInLocation("2006-01-02", rawDate, loc)
		if err != nil {
			return weatherd.Bundle{}, fmt.Errorf("parse open-meteo daily date %q: %w", rawDate, err)
		}
		sunrise, err := parseDailyClock(in.Daily.Sunrise, idx, loc)
		if err != nil {
			return weatherd.Bundle{}, err
		}
		sunset, err := parseDailyClock(in.Daily.Sunset, idx, loc)
		if err != nil {
			return weatherd.Bundle{}, err
		}
		point := weatherd.DailyForecastPoint{
			Date:    date.UTC(),
			Sunrise: sunrise.UTC(),
			Sunset:  sunset.UTC(),
			Condition: weatherd.WeatherCondition{
				WeatherCode: valueAtInt32(in.Daily.WeatherCode, idx),
			},
			DaylightDurationSeconds: valueAtFloat64(in.Daily.DaylightDuration, idx),
			Raw: weatherd.DailyValueSet{
				SunshineDurationSeconds: valueAtFloat64(in.Daily.SunshineDuration, idx),
				ShortwaveRadiationSum:   valueAtFloat64(in.Daily.ShortwaveRadiationSum, idx),
				UVIndexMax:              pointerAtFloat64(in.Daily.UVIndexMax, idx),
			},
		}
		point.Condition.WeatherText = WeatherCodeText(point.Condition.WeatherCode)
		point.Corrected = point.Raw
		out.Daily = append(out.Daily, point)
	}
	return out, nil
}

func parseDailyClock(values []string, idx int, loc *time.Location) (time.Time, error) {
	if idx >= len(values) {
		return time.Time{}, nil
	}
	t, err := time.ParseInLocation("2006-01-02T15:04", values[idx], loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse open-meteo daily clock %q: %w", values[idx], err)
	}
	return t, nil
}

func timeLocation(name string) (*time.Location, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("load open-meteo timezone %q: %w", name, err)
	}
	return loc, nil
}

func valueAtInt32(values []int32, idx int) int32 {
	if idx < 0 || idx >= len(values) {
		return 0
	}
	return values[idx]
}

func valueAtFloat64(values []float64, idx int) *float64 {
	if idx < 0 || idx >= len(values) {
		return nil
	}
	v := values[idx]
	return &v
}

func pointerAtFloat64(values []*float64, idx int) *float64 {
	if idx < 0 || idx >= len(values) {
		return nil
	}
	return values[idx]
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
