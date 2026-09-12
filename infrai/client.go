package infrai

import (
	"bytes"
	"context"
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

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

type CaptureInput struct {
	Title       string         `json:"title"`
	Message     string         `json:"message"`
	Exception   string         `json:"exception"`
	Level       string         `json:"level"`
	Tags        map[string]any `json:"tags"`
	Fingerprint []string       `json:"fingerprint"`
	Context     map[string]any `json:"context"`
}

type CaptureResult struct {
	EventID      string `json:"event_id"`
	ErrorGroupID string `json:"error_group_id"`
}

type GroupDetail struct {
	ErrorGroupID string `json:"error_group_id"`
	Status       string `json:"status"`
	Count        int    `json:"count"`
}

type envelope[T any] struct {
	OK       bool            `json:"ok"`
	Data     T               `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, http: httpClient, sleep: sleepContext}
}

// Capture calls infrai.errors.capture with a stable key so rate-limit retries remain idempotent.
func (c *Client) Capture(ctx context.Context, input CaptureInput, idempotencyKey string) (CaptureResult, error) {
	var result CaptureResult
	err := c.call(ctx, http.MethodPost, "/v1/errors/capture", input, idempotencyKey, &result)
	return result, err
}

func (c *Client) GroupDetail(ctx context.Context, groupID string) (GroupDetail, error) {
	var result GroupDetail
	path := "/v1/errors/group_detail/" + url.PathEscape(groupID)
	err := c.call(ctx, http.MethodGet, path, nil, "", &result)
	return result, err
}

func (c *Client) call(ctx context.Context, method, path string, payload any, idempotencyKey string, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		responseBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := retryDelay(resp.Header.Get("Retry-After"), attempt)
			if err := c.sleep(ctx, delay); err != nil {
				return err
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("Infrai HTTP status %d", resp.StatusCode)
		}

		var env envelope[json.RawMessage]
		if err := json.Unmarshal(responseBody, &env); err != nil {
			return fmt.Errorf("decode envelope: %w", err)
		}
		if !env.OK {
			if len(env.Error) == 0 || string(env.Error) == "null" {
				return errors.New("Infrai request was not accepted")
			}
			return fmt.Errorf("Infrai request: %s", env.Error)
		}
		if result != nil && len(env.Data) > 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, result); err != nil {
				return fmt.Errorf("decode data: %w", err)
			}
		}
		return nil
	}
	return errors.New("rate-limit retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
