package palworld

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPClient struct {
	baseURL, username, password string
	client                      *http.Client
}

func NewHTTPClient(baseURL, username, password string, timeout time.Duration) (*HTTPClient, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("invalid Palworld base URL %q", baseURL)
	}
	return &HTTPClient{baseURL: u.String(), username: username, password: password, client: &http.Client{Timeout: timeout}}, nil
}

func (c *HTTPClient) GetInfo(ctx context.Context) (Info, error) {
	var v Info
	return v, c.get(ctx, "/v1/api/info", &v)
}
func (c *HTTPClient) GetMetrics(ctx context.Context) (Metrics, error) {
	var v Metrics
	return v, c.get(ctx, "/v1/api/metrics", &v)
}
func (c *HTTPClient) GetPlayers(ctx context.Context) (Players, error) {
	var v Players
	if err := c.get(ctx, "/v1/api/players", &v); err != nil {
		return v, err
	}
	if err := ValidatePlayers(v); err != nil {
		return Players{}, err
	}
	return v, nil
}

func (c *HTTPClient) get(ctx context.Context, endpoint string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
	if err != nil {
		return fmt.Errorf("create Palworld request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("Palworld %s request failed: %w", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("Palworld %s returned HTTP %d", endpoint, resp.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 2<<20))
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("decode Palworld %s response: %w", endpoint, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("decode Palworld %s response: trailing JSON content", endpoint)
	}
	return nil
}
