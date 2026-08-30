package openmerge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var retryableStatuses = map[int]bool{408: true, 425: true, 429: true, 500: true, 502: true, 503: true, 504: true}

const sdkVersion = "0.3.0"

type Config struct {
	APIKey            string
	BaseURL           string
	HTTPClient        *http.Client
	MaxRetries        int
	RetryBaseDelay    time.Duration
	AllowInsecureHTTP bool
}

type Client struct {
	apiKey         string
	baseURL        *url.URL
	httpClient     *http.Client
	maxRetries     int
	retryBaseDelay time.Duration
}

func NewClient(config Config) (*Client, error) {
	if !strings.HasPrefix(config.APIKey, "om_") {
		return nil, errors.New("openmerge: APIKey must be an OpenMerge workspace key")
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://api.openmerge.dev"
	}
	baseURL, err := url.Parse(config.BaseURL)
	if err != nil || !baseURL.IsAbs() {
		return nil, errors.New("openmerge: BaseURL must be absolute")
	}
	loopback := baseURL.Hostname() == "localhost" || baseURL.Hostname() == "127.0.0.1" || baseURL.Hostname() == "::1"
	if baseURL.Scheme != "https" && !(baseURL.Scheme == "http" && (loopback || config.AllowInsecureHTTP)) {
		return nil, errors.New("openmerge: BaseURL must use HTTPS outside loopback development")
	}
	if baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, errors.New("openmerge: BaseURL cannot contain credentials, query, or fragment")
	}
	if config.MaxRetries < 0 || config.MaxRetries > 5 {
		return nil, errors.New("openmerge: MaxRetries must be between 0 and 5")
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 2
	}
	if config.RetryBaseDelay == 0 {
		config.RetryBaseDelay = 250 * time.Millisecond
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	baseURL.Path = strings.TrimSuffix(baseURL.Path, "/")
	return &Client{config.APIKey, baseURL, config.HTTPClient, config.MaxRetries, config.RetryBaseDelay}, nil
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, body any, idempotencyKey string, out any) error {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("openmerge: encode request: %w", err)
		}
	}
	requestID := fmt.Sprintf("go-%d", time.Now().UnixNano())
	safe := method == http.MethodGet || method == http.MethodHead || idempotencyKey != ""
	for attempt := 0; ; attempt++ {
		target := *c.baseURL
		target.Path += path
		target.RawQuery = query.Encode()
		request, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(encoded))
		if err != nil {
			return err
		}
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Authorization", "Bearer "+c.apiKey)
		request.Header.Set("X-Request-ID", requestID)
		request.Header.Set("X-OpenMerge-SDK", "go/"+sdkVersion)
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			request.Header.Set("Idempotency-Key", idempotencyKey)
		}
		response, err := c.httpClient.Do(request)
		if err != nil {
			if safe && attempt < c.maxRetries {
				if err := wait(ctx, c.retryBaseDelay*time.Duration(math.Pow(2, float64(attempt)))); err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("openmerge: request failed: %w", err)
		}
		if retryableStatuses[response.StatusCode] && safe && attempt < c.maxRetries {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if err := wait(ctx, retryDelay(response, c.retryBaseDelay*time.Duration(math.Pow(2, float64(attempt))))); err != nil {
				return err
			}
			continue
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return decodeAPIError(response)
		}
		if out == nil || response.StatusCode == http.StatusNoContent {
			_, _ = io.Copy(io.Discard, response.Body)
			return nil
		}
		if err := json.NewDecoder(response.Body).Decode(out); err != nil {
			return fmt.Errorf("openmerge: decode response: %w", err)
		}
		return nil
	}
}

func retryDelay(response *http.Response, fallback time.Duration) time.Duration {
	value := response.Header.Get("Retry-After")
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}

func wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func decodeAPIError(response *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	var details any
	_ = json.Unmarshal(raw, &details)
	message := fmt.Sprintf("API request failed with HTTP %d", response.StatusCode)
	if object, ok := details.(map[string]any); ok {
		if detail, ok := object["detail"].(string); ok {
			message = detail
		} else if value, ok := object["message"].(string); ok {
			message = value
		}
	}
	return &APIError{
		StatusCode: response.StatusCode,
		RequestID:  response.Header.Get("X-Request-ID"),
		Retryable:  retryableStatuses[response.StatusCode],
		Details:    details,
		Message:    message,
	}
}
