package tron

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
		logger: logger,
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
	startedAt := time.Now()
	statusCode := 0
	defer func() {
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
		return err
	}
	defer response.Body.Close()
	statusCode = response.StatusCode
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("TRON upstream returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(target); err != nil {
		return fmt.Errorf("decode TRON upstream response: %w", err)
	}
	return nil
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
