package tron

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"web3/internal/config"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
)

// Client owns the shared, read-only HTTP transport for TRONSCAN and TronGrid.
// Provider-specific request and response formats live in tronscan.go/usdt.go.
type Client struct {
	enabled      bool
	tronScanBase *url.URL
	tronGridBase *url.URL
	apiKey       string
	usdtContract string
	http         *http.Client
	logger       *zap.Logger
	statusMu     sync.RWMutex
	statuses     map[string]RuntimeStatus
	now          func() time.Time
}

const (
	ProviderTronScan = "TRONSCAN"
	ProviderTronGrid = "TRONGRID"
)

// RuntimeStatus 是上游最近一次成功和失败的内存快照，不包含请求参数或密钥。
type RuntimeStatus struct {
	Name          string
	LastSuccessAt *time.Time
	LastErrorCode string
	LastErrorAt   *time.Time
}

func NewClient(cfg config.TronApprovalConfig, logger *zap.Logger) (*Client, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	requestTimeout, err := time.ParseDuration(cfg.RequestTimeout)
	if err != nil || requestTimeout <= 0 {
		return nil, fmt.Errorf("invalid TRON request timeout")
	}
	tronScanBase, err := parseHTTPSBaseURL(cfg.TronScanBaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid TRONSCAN base URL: %w", err)
	}
	tronGridBase, err := parseHTTPSBaseURL(cfg.TronGridBaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid TronGrid base URL: %w", err)
	}
	if _, err := DecodeMainnetAddress(cfg.USDTContract); err != nil {
		return nil, fmt.Errorf("invalid TRON USDT contract: %w", err)
	}
	return &Client{
		enabled:      cfg.Enabled,
		tronScanBase: tronScanBase,
		tronGridBase: tronGridBase,
		apiKey:       strings.TrimSpace(cfg.APIKey),
		usdtContract: strings.TrimSpace(cfg.USDTContract),
		// Transport 从当前 gRPC 请求 context 创建 HTTP Client Span，并向上游注入 traceparent。
		http: &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
			Timeout:   requestTimeout,
		},
		logger:   logger,
		statuses: make(map[string]RuntimeStatus, 2),
		now:      time.Now,
	}, nil
}

func (c *Client) Enabled() bool { return c != nil && c.enabled }

func (c *Client) USDTContract() string {
	if c == nil {
		return ""
	}
	return c.usdtContract
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, body any, target any) (returnErr error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	provider := c.providerForURL(request.URL)
	failureCode := ""
	startedAt := time.Now()
	statusCode := 0
	defer func() {
		if provider != "" {
			c.recordRuntime(provider, failureCode)
		}
		fields := []zap.Field{
			zap.String("method", method),
			zap.String("provider", request.URL.Host),
			zap.String("path", request.URL.Path),
			zap.Int("status_code", statusCode),
			zap.Duration("duration", time.Since(startedAt)),
		}
		if returnErr != nil {
			c.logger.Warn("TRON upstream request failed", append(fields, zap.Error(returnErr))...)
			return
		}
		c.logger.Debug("TRON upstream request completed", fields...)
	}()
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		request.Header.Set("TRON-PRO-API-KEY", c.apiKey)
	}
	response, err := c.http.Do(request)
	if err != nil {
		failureCode = classifyRequestError(err)
		return err
	}
	defer response.Body.Close()
	statusCode = response.StatusCode
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		failureCode = classifyHTTPStatus(response.StatusCode)
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("TRON upstream returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(target); err != nil {
		failureCode = "INVALID_UPSTREAM_RESPONSE"
		return fmt.Errorf("decode TRON upstream response: %w", err)
	}
	return nil
}

// RuntimeStatuses 返回固定顺序的状态副本，调用方不能修改 Client 内部状态。
func (c *Client) RuntimeStatuses() []RuntimeStatus {
	if c == nil {
		return nil
	}
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	result := make([]RuntimeStatus, 0, 2)
	for _, name := range []string{ProviderTronScan, ProviderTronGrid} {
		status := c.statuses[name]
		status.Name = name
		status.LastSuccessAt = cloneTime(status.LastSuccessAt)
		status.LastErrorAt = cloneTime(status.LastErrorAt)
		result = append(result, status)
	}
	return result
}

func (c *Client) providerForURL(value *url.URL) string {
	if value == nil {
		return ""
	}
	switch value.Host {
	case c.tronScanBase.Host:
		return ProviderTronScan
	case c.tronGridBase.Host:
		return ProviderTronGrid
	default:
		return ""
	}
}

func (c *Client) recordRuntime(provider, failureCode string) {
	observedAt := c.now().UTC()
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	status := c.statuses[provider]
	status.Name = provider
	if failureCode == "" {
		status.LastSuccessAt = &observedAt
	} else {
		status.LastErrorCode = failureCode
		status.LastErrorAt = &observedAt
	}
	c.statuses[provider] = status
}

func classifyRequestError(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "UPSTREAM_TIMEOUT"
	case errors.Is(err, context.Canceled):
		return "REQUEST_CANCELED"
	default:
		return "UPSTREAM_UNAVAILABLE"
	}
}

func classifyHTTPStatus(statusCode int) string {
	switch {
	case statusCode == http.StatusRequestTimeout || statusCode == http.StatusGatewayTimeout:
		return "UPSTREAM_TIMEOUT"
	case statusCode == http.StatusTooManyRequests:
		return "RATE_LIMITED"
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return "AUTHENTICATION_FAILED"
	case statusCode >= http.StatusInternalServerError:
		return "UPSTREAM_UNAVAILABLE"
	default:
		return "UPSTREAM_HTTP_ERROR"
	}
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func resolve(base *url.URL, path string) *url.URL {
	copyURL := *base
	copyURL.Path = strings.TrimRight(copyURL.Path, "/") + path
	copyURL.RawQuery = ""
	return &copyURL
}

func parseHTTPSBaseURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("base URL must be an HTTPS origin without user info")
	}
	return parsed, nil
}
