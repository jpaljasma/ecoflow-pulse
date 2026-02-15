package ecoflow

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"
)

// Request describes one EcoFlow API call.
//
// Path may be absolute or relative to Config.BaseURL.
type Request struct {
	Method  string
	Path    string
	Query   url.Values
	Body    any
	Headers http.Header
}

// Response contains raw response metadata and payload bytes.
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// Client is a reusable, concurrency-safe EcoFlow API client.
type Client struct {
	baseURL             *url.URL
	httpClient          *http.Client
	credentialsProvider CredentialsProvider
	signer              Signer
	retryPolicy         RetryPolicy
	userAgent           string

	nonce func() (string, error)
	now   func() time.Time

	tracer                 Tracer
	metrics                *clientMetrics
	logger                 *slog.Logger
	advancedDebugTelemetry bool
	debugLogHeaders        bool
	compression            clientCompressionRuntime
}

// NewClient constructs a client from explicit configuration.
func NewClient(cfg Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	parsedBaseURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		transport := cfg.Transport
		if transport == nil {
			transport = NewTunedTransport(cfg.TransportTuning)
		}
		httpClient = &http.Client{
			Transport: transport,
			Timeout:   cfg.RequestTimeout,
		}
	}

	obs := cfg.Observability
	defaultObs := defaultObservability()
	if obs.Tracer == nil {
		obs.Tracer = defaultObs.Tracer
	}
	if obs.Meter == nil {
		obs.Meter = defaultObs.Meter
	}

	metrics, err := newClientMetrics(obs.Meter)
	if err != nil {
		return nil, fmt.Errorf("init metrics: %w", err)
	}

	logging := cfg.Logging
	if logging.Logger == nil {
		if logging.Debug {
			logging.Logger = defaultLoggingOptions(EnvironmentDev).Logger
		} else {
			logging.Logger = defaultLoggingOptions(EnvironmentProd).Logger
		}
	}
	compressionRuntime, err := newClientCompressionRuntime(cfg.Compression)
	if err != nil {
		return nil, fmt.Errorf("init compression: %w", err)
	}

	return &Client{
		baseURL:                parsedBaseURL,
		httpClient:             httpClient,
		credentialsProvider:    cfg.CredentialsProvider,
		signer:                 cfg.Signer,
		retryPolicy:            cfg.RetryPolicy,
		userAgent:              cfg.UserAgent,
		nonce:                  generateNonce,
		now:                    time.Now,
		tracer:                 obs.Tracer,
		metrics:                metrics,
		logger:                 logging.Logger,
		advancedDebugTelemetry: logging.AdvancedDebugTelemetry,
		debugLogHeaders:        logging.DebugLogHeaders,
		compression:            compressionRuntime,
	}, nil
}

// NewClientFromEnvironment constructs a client from process env/.env values.
func NewClientFromEnvironment() (*Client, error) {
	cfg, err := ConfigFromEnvironment()
	if err != nil {
		return nil, err
	}
	return NewClient(cfg)
}

// Do sends a signed request to EcoFlow and returns the raw response.
//
// Retry behavior is governed by Config.RetryPolicy.
func (c *Client) Do(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(req.Method) == "" {
		return Response{}, errors.New("request method is required")
	}
	if strings.TrimSpace(req.Path) == "" {
		return Response{}, errors.New("request path is required")
	}

	credentials, err := c.credentialsProvider.Credentials(ctx)
	if err != nil {
		return Response{}, err
	}

	bodyBytes, err := marshalBody(req.Body)
	if err != nil {
		return Response{}, err
	}
	preparedBodyBytes, requestContentEncoding, err := c.compression.maybeCompressBody(bodyBytes)
	if err != nil {
		return Response{}, err
	}

	u, err := c.resolveURL(req.Path, req.Query)
	if err != nil {
		return Response{}, err
	}

	method := strings.ToUpper(req.Method)
	start := c.now()
	ctx, span := c.tracer.Start(ctx, "ecoflow.http.request")
	defer span.End()

	var lastErr error
	for attempt := 1; attempt <= c.retryPolicy.MaxAttempts; attempt++ {
		attemptStart := c.now()
		httpReq, signErr := c.buildSignedRequest(
			ctx,
			method,
			u,
			req.Headers,
			preparedBodyBytes,
			req.Body,
			credentials,
			requestContentEncoding,
			c.compression.responseAcceptEncodingHeader(),
		)
		if signErr != nil {
			return Response{}, signErr
		}
		debugEnabled := c.logger != nil && c.logger.Enabled(ctx, slog.LevelDebug)
		requestBytes := 0
		requestCompression := ""
		runtimeStart := runtimeSnapshot{}
		if debugEnabled {
			if c.advancedDebugTelemetry {
				requestBytes = estimateRequestSize(httpReq, len(preparedBodyBytes))
				requestCompression = compressionOrIdentity(httpReq.Header.Get("Content-Encoding"))
				runtimeStart = captureRuntimeSnapshot(attemptStart)
			}
			c.logOutgoingRequest(
				ctx,
				httpReq,
				attempt,
				len(bodyBytes),
				requestBytes,
				requestCompression,
				httpReq.Header.Get("Accept-Encoding"),
			)
		}

		response, requestErr := c.httpClient.Do(httpReq)
		if c.retryPolicy.shouldRetry(method, response, requestErr) && attempt < c.retryPolicy.MaxAttempts {
			attrs := requestAttributes(method, httpReq.URL.Path, statusCodeOf(response), attempt)
			c.metrics.recordRetry(ctx, attrs)
			_ = drainAndClose(response)
			delay := c.retryPolicy.delayForAttempt(c.now(), response, attempt)
			c.logRetryDecision(ctx, httpReq, attempt, statusCodeOf(response), requestErr, delay, requestBytes, requestCompression, responseCompression(response))
			if err := sleepWithContext(ctx, delay); err != nil {
				annotateSpanResult(span, statusCodeOf(response), err)
				return Response{}, err
			}
			lastErr = requestErr
			continue
		}

		if requestErr != nil {
			attrs := requestAttributes(method, httpReq.URL.Path, 0, attempt)
			c.metrics.recordRequest(ctx, attrs)
			c.metrics.recordFailure(ctx, attrs)
			c.metrics.recordLatency(ctx, c.now().Sub(start), attrs)
			c.logRequestError(
				ctx,
				httpReq,
				attempt,
				requestErr,
				c.now().Sub(attemptStart),
				requestBytes,
				requestCompression,
				runtimeStart,
			)
			annotateSpanResult(span, 0, requestErr)
			return Response{}, requestErr
		}

		decodedBody, responseCompressionUsed, decodeErr := c.compression.decodeResponseBody(response)
		if decodeErr != nil {
			_ = drainAndClose(response)
			attrs := requestAttributes(method, httpReq.URL.Path, response.StatusCode, attempt)
			c.metrics.recordRequest(ctx, attrs)
			c.metrics.recordFailure(ctx, attrs)
			c.metrics.recordLatency(ctx, c.now().Sub(start), attrs)
			annotateSpanResult(span, response.StatusCode, decodeErr)
			return Response{}, decodeErr
		}

		body, readErr := io.ReadAll(decodedBody)
		_ = decodedBody.Close()
		if readErr != nil {
			attrs := requestAttributes(method, httpReq.URL.Path, response.StatusCode, attempt)
			c.metrics.recordRequest(ctx, attrs)
			c.metrics.recordFailure(ctx, attrs)
			c.metrics.recordLatency(ctx, c.now().Sub(start), attrs)
			annotateSpanResult(span, response.StatusCode, readErr)
			return Response{}, readErr
		}

		result := Response{
			StatusCode: response.StatusCode,
			Headers:    response.Header.Clone(),
			Body:       body,
		}
		c.logResponse(
			ctx,
			httpReq,
			response.Header,
			response.StatusCode,
			attempt,
			c.now().Sub(attemptStart),
			requestBytes,
			len(body),
			requestCompression,
			responseCompressionUsed,
			runtimeStart,
		)
		attrs := requestAttributes(method, httpReq.URL.Path, response.StatusCode, attempt)
		c.metrics.recordRequest(ctx, attrs)
		c.metrics.recordLatency(ctx, c.now().Sub(start), attrs)
		if response.StatusCode >= 400 {
			httpErr := &HTTPError{
				StatusCode: response.StatusCode,
				Body:       body,
			}
			c.metrics.recordFailure(ctx, attrs)
			annotateSpanResult(span, response.StatusCode, httpErr)
			return result, httpErr
		}

		annotateSpanResult(span, response.StatusCode, nil)
		return result, nil
	}

	retryErr := &RetryExhaustedError{
		Attempts: c.retryPolicy.MaxAttempts,
		LastErr:  lastErr,
	}
	annotateSpanResult(span, 0, retryErr)
	return Response{}, retryErr
}

// DoJSON sends a signed request and unmarshals a successful JSON response into out.
func (c *Client) DoJSON(ctx context.Context, req Request, out any) (Response, error) {
	response, err := c.Do(ctx, req)
	if err != nil {
		return response, err
	}
	if out == nil || len(response.Body) == 0 {
		return response, nil
	}
	if err := json.Unmarshal(response.Body, out); err != nil {
		return response, fmt.Errorf("decode response JSON: %w", err)
	}
	return response, nil
}

func (c *Client) buildSignedRequest(
	ctx context.Context,
	method string,
	endpoint *url.URL,
	headers http.Header,
	bodyBytes []byte,
	bodyForSigning any,
	credentials Credentials,
	requestContentEncoding string,
	acceptEncoding string,
) (*http.Request, error) {
	var body io.Reader
	if len(bodyBytes) > 0 {
		body = bytes.NewReader(bodyBytes)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if headers != nil {
		request.Header = headers.Clone()
	}
	if request.Header == nil {
		request.Header = make(http.Header)
	}

	if request.Header.Get("User-Agent") == "" && c.userAgent != "" {
		request.Header.Set("User-Agent", c.userAgent)
	}
	if len(bodyBytes) > 0 && request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	if requestContentEncoding != "" && request.Header.Get("Content-Encoding") == "" {
		request.Header.Set("Content-Encoding", requestContentEncoding)
	}
	if request.Header.Get("Accept") == "" {
		request.Header.Set("Accept", "application/json")
	}
	if acceptEncoding != "" && request.Header.Get("Accept-Encoding") == "" {
		request.Header.Set("Accept-Encoding", acceptEncoding)
	}

	nonce, err := c.nonce()
	if err != nil {
		return nil, err
	}
	timestampMillis := c.now().UnixMilli()

	signResult, err := c.signer.Sign(ctx, SignInput{
		Credentials:     credentials,
		Nonce:           nonce,
		TimestampMillis: timestampMillis,
		Query:           request.URL.Query(),
		Body:            bodyForSigning,
	})
	if err != nil {
		return nil, fmt.Errorf("sign request: %w", err)
	}

	request.Header.Set(HeaderAccessKey, credentials.AccessKey)
	request.Header.Set(HeaderNonce, nonce)
	request.Header.Set(HeaderTimestamp, fmt.Sprintf("%d", timestampMillis))
	request.Header.Set(HeaderSignature, signResult.Signature)

	return request, nil
}

func (c *Client) resolveURL(path string, query url.Values) (*url.URL, error) {
	u, err := c.baseURL.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path %q: %w", path, err)
	}
	q := u.Query()
	for key, values := range query {
		for _, value := range values {
			q.Add(key, value)
		}
	}
	u.RawQuery = q.Encode()
	return u, nil
}

func marshalBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	switch v := body.(type) {
	case []byte:
		return v, nil
	case json.RawMessage:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		return encoded, nil
	}
}

func drainAndClose(response *http.Response) error {
	if response == nil || response.Body == nil {
		return nil
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8*1024))
	return response.Body.Close()
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func generateNonce() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	return hex.EncodeToString(data), nil
}

func statusCodeOf(response *http.Response) int {
	if response == nil {
		return 0
	}
	return response.StatusCode
}

func (c *Client) logOutgoingRequest(
	ctx context.Context,
	req *http.Request,
	attempt int,
	bodyBytes int,
	requestBytes int,
	requestCompression string,
	acceptEncoding string,
) {
	if c.logger == nil || !c.logger.Enabled(ctx, slog.LevelDebug) {
		return
	}
	args := []any{
		slog.String("method", req.Method),
		slog.String("scheme", req.URL.Scheme),
		slog.String("host", req.URL.Host),
		slog.String("path", req.URL.Path),
		slog.String("query", req.URL.RawQuery),
		slog.Int("attempt", attempt),
		slog.Int("max_attempts", c.retryPolicy.MaxAttempts),
		slog.Int("body_bytes", bodyBytes),
		slog.String("accept_encoding", strings.ToLower(strings.TrimSpace(acceptEncoding))),
		slog.String("access_key_suffix", accessKeySuffix(req.Header.Get(HeaderAccessKey))),
		slog.String("nonce", req.Header.Get(HeaderNonce)),
	}
	if c.advancedDebugTelemetry {
		current := captureRuntimeSnapshot(c.now())
		args = append(
			args,
			slog.Int("request_bytes_sent_estimate", requestBytes),
			slog.String("request_compression", requestCompression),
			slog.Uint64("mem_alloc_bytes", current.memAllocBytes),
			slog.Uint64("mem_sys_bytes", current.memSysBytes),
			slog.Uint64("heap_inuse_bytes", current.heapInuse),
			slog.Float64("mem_load_pct", memoryLoadPercent(current.memAllocBytes, current.memSysBytes)),
			slog.Int("gomaxprocs", runtime.GOMAXPROCS(0)),
			slog.Int("goroutines", runtime.NumGoroutine()),
		)
	}
	if c.debugLogHeaders {
		args = append(args, slog.Any("request_headers", headersForLogging(req.Header)))
	}
	c.logger.DebugContext(ctx, "ecoflow outgoing http request", args...)
}

func (c *Client) logRetryDecision(
	ctx context.Context,
	req *http.Request,
	attempt int,
	statusCode int,
	requestErr error,
	backoff time.Duration,
	requestBytes int,
	requestCompression string,
	responseCompression string,
) {
	if c.logger == nil || !c.logger.Enabled(ctx, slog.LevelDebug) {
		return
	}
	args := []any{
		slog.String("method", req.Method),
		slog.String("path", req.URL.Path),
		slog.Int("attempt", attempt),
		slog.Int("status_code", statusCode),
		slog.String("error", errorString(requestErr)),
		slog.Duration("backoff", backoff),
	}
	if c.advancedDebugTelemetry {
		args = append(
			args,
			slog.Int("request_bytes_sent_estimate", requestBytes),
			slog.String("request_compression", requestCompression),
			slog.String("response_compression", responseCompression),
		)
	}
	c.logger.DebugContext(ctx, "ecoflow retry scheduled", args...)
}

func (c *Client) logRequestError(
	ctx context.Context,
	req *http.Request,
	attempt int,
	requestErr error,
	latency time.Duration,
	requestBytes int,
	requestCompression string,
	runtimeStart runtimeSnapshot,
) {
	if c.logger == nil || !c.logger.Enabled(ctx, slog.LevelDebug) {
		return
	}
	args := []any{
		slog.String("method", req.Method),
		slog.String("path", req.URL.Path),
		slog.Int("attempt", attempt),
		slog.String("error", errorString(requestErr)),
	}
	if c.advancedDebugTelemetry {
		load := computeRuntimeLoad(runtimeStart, captureRuntimeSnapshot(c.now()), latency)
		args = append(
			args,
			slog.Duration("round_trip_latency", latency),
			slog.Int("request_bytes_sent_estimate", requestBytes),
			slog.Int("response_bytes_received", 0),
			slog.String("request_compression", requestCompression),
			slog.String("response_compression", "unknown"),
			slog.Uint64("mem_alloc_bytes", load.memAllocBytes),
			slog.Uint64("mem_sys_bytes", load.memSysBytes),
			slog.Uint64("heap_inuse_bytes", load.heapInuse),
			slog.Float64("mem_load_pct", load.memLoadPct),
			slog.Int("gomaxprocs", load.goMaxProcs),
			slog.Int("goroutines", load.goroutines),
			slog.Float64("cpu_load_pct", cpuLoadOrNegative(load)),
		)
	}
	if c.debugLogHeaders {
		args = append(args, slog.Any("request_headers", headersForLogging(req.Header)))
	}
	c.logger.DebugContext(ctx, "ecoflow request failed", args...)
}

func (c *Client) logResponse(
	ctx context.Context,
	req *http.Request,
	responseHeaders http.Header,
	statusCode int,
	attempt int,
	latency time.Duration,
	requestBytes int,
	responseBytes int,
	requestCompression string,
	responseCompression string,
	runtimeStart runtimeSnapshot,
) {
	if c.logger == nil || !c.logger.Enabled(ctx, slog.LevelDebug) {
		return
	}
	args := []any{
		slog.String("method", req.Method),
		slog.String("path", req.URL.Path),
		slog.Int("status_code", statusCode),
		slog.Int("attempt", attempt),
	}
	if c.advancedDebugTelemetry {
		load := computeRuntimeLoad(runtimeStart, captureRuntimeSnapshot(c.now()), latency)
		args = append(
			args,
			slog.Int("request_bytes_sent_estimate", requestBytes),
			slog.Int("response_bytes_received", responseBytes),
			slog.Duration("round_trip_latency", latency),
			slog.String("request_compression", requestCompression),
			slog.String("response_compression", responseCompression),
			slog.Uint64("mem_alloc_bytes", load.memAllocBytes),
			slog.Uint64("mem_sys_bytes", load.memSysBytes),
			slog.Uint64("heap_inuse_bytes", load.heapInuse),
			slog.Float64("mem_load_pct", load.memLoadPct),
			slog.Int("gomaxprocs", load.goMaxProcs),
			slog.Int("goroutines", load.goroutines),
			slog.Float64("cpu_load_pct", cpuLoadOrNegative(load)),
		)
	}
	if c.debugLogHeaders {
		args = append(
			args,
			slog.Any("request_headers", headersForLogging(req.Header)),
			slog.Any("response_headers", headersForLogging(responseHeaders)),
		)
	}
	c.logger.DebugContext(ctx, "ecoflow response received", args...)
}

func accessKeySuffix(accessKey string) string {
	trimmed := strings.TrimSpace(accessKey)
	if len(trimmed) <= 4 {
		return trimmed
	}
	return trimmed[len(trimmed)-4:]
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func estimateRequestSize(req *http.Request, bodyBytes int) int {
	if req == nil {
		return bodyBytes
	}

	uri := req.URL.RequestURI()
	if uri == "" {
		uri = req.URL.Path
	}

	total := 0
	total += len(req.Method) + 1 + len(uri) + len(" HTTP/1.1\r\n")

	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	if host != "" {
		total += len("Host: ") + len(host) + len("\r\n")
	}
	for key, values := range req.Header {
		for _, value := range values {
			total += len(key) + len(": ") + len(value) + len("\r\n")
		}
	}
	total += len("\r\n")
	total += bodyBytes
	return total
}

func compressionOrIdentity(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "identity"
	}
	return value
}

func responseCompression(response *http.Response) string {
	if response == nil {
		return "unknown"
	}
	if value := strings.TrimSpace(response.Header.Get("Content-Encoding")); value != "" {
		return strings.ToLower(value)
	}
	if response.Uncompressed {
		return "gzip(auto-decoded)"
	}
	return "identity"
}

func cpuLoadOrNegative(load runtimeLoad) float64 {
	if !load.cpuKnown {
		return -1
	}
	return load.cpuLoadPercent
}

func memoryLoadPercent(allocBytes, sysBytes uint64) float64 {
	if sysBytes == 0 {
		return 0
	}
	return float64(allocBytes) * 100 / float64(sysBytes)
}

func headersForLogging(headers http.Header) map[string][]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string][]string, len(headers))
	for key, values := range headers {
		if len(values) == 0 {
			out[key] = nil
			continue
		}
		copied := make([]string, len(values))
		copy(copied, values)
		out[key] = copied
	}
	return out
}
