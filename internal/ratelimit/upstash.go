package ratelimit

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

type UpstashClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewUpstashClient(restURL, token string) (*UpstashClient, error) {
	restURL = strings.TrimRight(strings.TrimSpace(restURL), "/")
	token = strings.TrimSpace(token)
	if restURL == "" || token == "" {
		return nil, fmt.Errorf("upstash redis url and token are required")
	}

	return &UpstashClient{
		baseURL: restURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}, nil
}

func (c *UpstashClient) GetInt(ctx context.Context, key string) (int, error) {
	result, err := c.command(ctx, "get", key)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, nil
	}

	switch typed := result.(type) {
	case float64:
		return int(typed), nil
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case string:
		if typed == "" {
			return 0, nil
		}
		var parsed int
		if _, err := fmt.Sscanf(typed, "%d", &parsed); err != nil {
			return 0, fmt.Errorf("parse redis get result: %w", err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected redis get result type %T", result)
	}
}

func (c *UpstashClient) Incr(ctx context.Context, key string) (int, error) {
	result, err := c.command(ctx, "incr", key)
	if err != nil {
		return 0, err
	}

	switch typed := result.(type) {
	case float64:
		return int(typed), nil
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	default:
		return 0, fmt.Errorf("unexpected redis incr result type %T", result)
	}
}

func (c *UpstashClient) Expire(ctx context.Context, key string, ttlSeconds int) error {
	_, err := c.command(ctx, "expire", key, fmt.Sprintf("%d", ttlSeconds))
	return err
}

func (c *UpstashClient) command(ctx context.Context, parts ...string) (any, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("redis command requires at least one part")
	}

	endpoint := c.baseURL
	for _, part := range parts {
		endpoint += "/" + url.PathEscape(part)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build upstash request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstash request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read upstash response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstash error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Result any `json:"result"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode upstash response: %w", err)
	}
	if payload.Error != "" {
		return nil, fmt.Errorf("upstash command error: %s", payload.Error)
	}

	return payload.Result, nil
}
