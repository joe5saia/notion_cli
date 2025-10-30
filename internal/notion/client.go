// Package notion provides a resilient Notion REST API client used by notionctl.
package notion

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultBaseURL             = "https://api.notion.com/v1"
	defaultMaxRetries          = 5
	defaultBackoffInitialDelay = 500 * time.Millisecond
	defaultNotionVersion       = "2025-09-03"

	limiterRatePerSecond = 3
	limiterBurstTokens   = 6

	backoffFactor       = 2.0
	maxBackoffDelay     = 30 * time.Second
	jitterLowerBound    = 0.8
	jitterUpperBound    = 1.2
	float64MantissaBits = 53
	userAgent           = "notionctl/0.1"
)

// ClientConfig configures the Notion client.
type ClientConfig struct {
	HTTPClient    *http.Client
	Token         string
	BaseURL       string
	NotionVersion string
	BackoffBase   time.Duration
	MaxRetries    int
}

// Client performs authenticated requests to the Notion REST API with retries.
type Client struct {
	http    *http.Client
	baseURL *url.URL
	limiter *rate.Limiter
	jitter  func() float64
	sleep   func(time.Duration)
	cfg     ClientConfig
}

// NewClient constructs a Client with production-safe defaults.
func NewClient(cfg ClientConfig) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second, //nolint:mnd // default HTTP client timeout
		}
	}

	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = defaultMaxRetries
	}
	if cfg.BackoffBase <= 0 {
		cfg.BackoffBase = defaultBackoffInitialDelay
	}
	if cfg.NotionVersion == "" {
		cfg.NotionVersion = defaultNotionVersion
	}

	base := cfg.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	parsed, err := url.Parse(base)
	if err != nil {
		panic(fmt.Sprintf("invalid Notion base URL %q: %v", base, err))
	}

	return &Client{
		cfg:     cfg,
		http:    httpClient,
		baseURL: parsed,
		limiter: rate.NewLimiter(rate.Limit(limiterRatePerSecond), limiterBurstTokens),
		sleep:   time.Sleep,
		jitter:  func() float64 { return randomFloat64(jitterLowerBound, jitterUpperBound) },
	}
}

// Do exposes the low-level request helper for advanced use-cases.
func (c *Client) Do(ctx context.Context, method, path string, body any, out any) error {
	req, payload, err := c.prepareRequest(ctx, method, path, body)
	if err != nil {
		return err
	}

	return c.executeWithRetries(ctx, req, payload, out)
}

func (c *Client) executeWithRetries(ctx context.Context, req *http.Request, payload []byte, out any) error {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if err := c.beforeAttempt(ctx, attempt, req, payload); err != nil {
			return err
		}

		resp, reqErr := c.http.Do(req)
		decision, closed := c.evaluateResponse(ctx, resp, reqErr, out)
		decision = c.finalizeDecision(resp, decision, closed)
		if decision.err != nil {
			lastErr = decision.err
		}
		if !decision.retry {
			return decision.err
		}
		c.backoff(attempt, decision.retryAfter)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("exhausted retries after %d attempts", c.cfg.MaxRetries+1)
	}
	return lastErr
}

// do is retained for internal callers to avoid recursive wrappers.
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	return c.Do(ctx, method, path, body, out)
}

func (c *Client) beforeAttempt(ctx context.Context, attempt int, req *http.Request, payload []byte) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit wait: %w", err)
	}
	if attempt == 0 || payload == nil {
		return nil
	}
	return c.resetRequestBody(req)
}

func (c *Client) prepareRequest(
	ctx context.Context,
	method string,
	requestPath string,
	body any,
) (*http.Request, []byte, error) {
	target, err := c.resolve(requestPath)
	if err != nil {
		return nil, nil, err
	}

	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("encode request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(payload))
	if err != nil {
		return nil, nil, fmt.Errorf("build request: %w", err)
	}
	if payload != nil {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(payload)), nil
		}
		req.ContentLength = int64(len(payload))
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("Notion-Version", c.cfg.NotionVersion)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	return req, payload, nil
}

func (c *Client) resetRequestBody(req *http.Request) error {
	if req.GetBody == nil {
		return errors.New("request body cannot be reset")
	}
	body, err := req.GetBody()
	if err != nil {
		return fmt.Errorf("reset request body: %w", err)
	}
	req.Body = body
	return nil
}

type responseDecision struct {
	err        error
	retryAfter time.Duration
	retry      bool
}

func (c *Client) evaluateResponse(
	ctx context.Context,
	resp *http.Response,
	reqErr error,
	out any,
) (responseDecision, bool) {
	if reqErr != nil {
		return c.handleRequestError(ctx, reqErr), true
	}
	if resp == nil {
		return responseDecision{retry: true, err: errors.New("notion: nil response")}, true
	}
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return c.handleSuccess(resp, out)
	}
	return c.handleFailure(resp)
}

func (c *Client) handleRequestError(ctx context.Context, reqErr error) responseDecision {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return responseDecision{err: fmt.Errorf("request context: %w", ctxErr)}
	}
	return responseDecision{retry: true, err: fmt.Errorf("do request: %w", reqErr)}
}

func (c *Client) handleSuccess(resp *http.Response, out any) (responseDecision, bool) {
	if out != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return responseDecision{err: fmt.Errorf("decode response: %w", err)}, false
		}
	}
	return responseDecision{}, false
}

func (c *Client) handleFailure(resp *http.Response) (responseDecision, bool) {
	if isRetryableStatus(resp.StatusCode) {
		return responseDecision{retry: true, retryAfter: parseRetryAfter(resp), err: decodeError(resp)}, true
	}
	return responseDecision{retry: false, err: decodeError(resp)}, true
}

func (c *Client) finalizeDecision(resp *http.Response, decision responseDecision, closed bool) responseDecision {
	if closed || resp == nil || resp.Body == nil {
		return decision
	}

	if closeErr := resp.Body.Close(); closeErr != nil {
		wrapped := fmt.Errorf("close response body: %w", closeErr)
		if decision.err != nil {
			decision.err = errors.Join(decision.err, wrapped)
		} else {
			decision.err = wrapped
		}
		decision.retry = false
	}
	return decision
}

func (c *Client) backoff(attempt int, retryAfter time.Duration) {
	if retryAfter > 0 {
		c.sleep(retryAfter)
		return
	}

	delay := float64(c.cfg.BackoffBase) * math.Pow(backoffFactor, float64(attempt)) * c.jitter()
	backoff := time.Duration(delay)
	if backoff > maxBackoffDelay {
		backoff = maxBackoffDelay
	}
	c.sleep(backoff)
}

func (c *Client) resolve(requestPath string) (string, error) {
	if strings.HasPrefix(requestPath, "http://") || strings.HasPrefix(requestPath, "https://") {
		return requestPath, nil
	}
	target, err := c.baseURL.Parse(strings.TrimPrefix(requestPath, "/"))
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", requestPath, err)
	}
	return target.String(), nil
}

func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func parseRetryAfter(resp *http.Response) time.Duration {
	retryAfter := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if retryAfter == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if ts, err := time.Parse(time.RFC1123, retryAfter); err == nil {
		return time.Until(ts)
	}
	return 0
}

func randomFloat64(min, max float64) float64 {
	if max <= min {
		return min
	}
	diff := max - min
	limit := int64(1 << float64MantissaBits)
	n, err := rand.Int(rand.Reader, big.NewInt(limit))
	if err != nil {
		return min
	}
	fraction := float64(n.Int64()) / float64(limit)
	return min + diff*fraction
}

// WithLimiter allows overriding the rate limiter (used by tests).
func (c *Client) WithLimiter(l *rate.Limiter) {
	if l != nil {
		c.limiter = l
	}
}

// WithSleeper injects a sleep function (tests may stub to avoid waiting).
func (c *Client) WithSleeper(s func(time.Duration)) {
	if s != nil {
		c.sleep = s
	}
}

// WithJitter injects a custom jitter provider.
func (c *Client) WithJitter(j func() float64) {
	if j != nil {
		c.jitter = j
	}
}

// SetToken updates the bearer token.
func (c *Client) SetToken(token string) {
	c.cfg.Token = token
}

// Token returns the configured bearer token.
func (c *Client) Token() string {
	return c.cfg.Token
}

// NotionVersion exposes the configured Notion API version.
func (c *Client) NotionVersion() string {
	return c.cfg.NotionVersion
}

// SetNotionVersion updates the Notion API version header.
func (c *Client) SetNotionVersion(version string) {
	if version == "" {
		version = defaultNotionVersion
	}
	c.cfg.NotionVersion = version
}
