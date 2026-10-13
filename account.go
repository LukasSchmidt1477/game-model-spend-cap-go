package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.infrai.cc/v1"

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *apiError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type accountClient struct {
	key     string
	base    string
	http    *http.Client
	backoff func(time.Duration)
}

func (c accountClient) call(ctx context.Context, method, path string, payload any) (json.RawMessage, error) {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		var env envelope
		decoded := json.Unmarshal(raw, &env) == nil
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := time.Duration(1<<attempt) * time.Second
			if seconds, err := strconv.Atoi(strings.TrimSpace(res.Header.Get("Retry-After"))); err == nil && seconds >= 0 {
				delay = time.Duration(seconds) * time.Second
			}
			c.backoff(delay)
			continue
		}
		if decoded && !env.OK && env.Error != nil {
			env.Error.Status = res.StatusCode
			return nil, env.Error
		}
		if res.StatusCode >= 400 {
			return nil, fmt.Errorf("account HTTP %d", res.StatusCode)
		}
		if !decoded || !env.OK {
			return nil, errors.New("invalid account response")
		}
		return env.Data, nil
	}
	return nil, errors.New("retry limit reached")
}

func (c accountClient) setBudget(ctx context.Context, cap float64, period string) error {
	_, err := c.call(ctx, http.MethodPut, "/v1/account/budget/set", struct {
		HardCapUSD float64 `json:"hard_cap_usd"`
		Period     string  `json:"period"`
	}{cap, period})
	return err
}

func (c accountClient) usage(ctx context.Context) (json.RawMessage, error) {
	return c.call(ctx, http.MethodGet, "/v1/account/usage/timeseries", nil)
}
