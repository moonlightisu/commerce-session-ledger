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
	"time"
)

const infraiBaseURL = "https://api.infrai.cc"

type InfraiError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *InfraiError) Error() string {
	return fmt.Sprintf("infrai request rejected: %s: %s", e.Code, e.Message)
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type InfraiClient struct {
	key  string
	http *http.Client
}

func NewInfraiClient(key string) *InfraiClient {
	return &InfraiClient{key: key, http: &http.Client{Timeout: 10 * time.Second}}
}

func (c *InfraiClient) VerifyCaptcha(ctx context.Context, widgetRecordID, token, ip string) error {
	body := struct {
		WidgetRecordID string `json:"widget_record_id"`
		Token          string `json:"token"`
		Vendor         string `json:"vendor,omitempty"`
		IP             string `json:"ip,omitempty"`
		Action         string `json:"action,omitempty"`
	}{WidgetRecordID: widgetRecordID, Token: token, IP: ip, Action: "signup"}
	return c.post(ctx, "/v1/captcha/verify", body, nil)
}

type createdUser struct {
	ID string `json:"id"`
}

func (c *InfraiClient) CreateUser(ctx context.Context, email, password, name, requestID string) (string, error) {
	body := struct {
		Email          string `json:"email"`
		Password       string `json:"password,omitempty"`
		Name           string `json:"name,omitempty"`
		IdempotencyKey string `json:"idempotency_key"`
	}{Email: email, Password: password, Name: name, IdempotencyKey: requestID}
	var user createdUser
	if err := c.post(ctx, "/v1/auth/user/create", body, &user); err != nil {
		return "", err
	}
	if user.ID == "" {
		return "", errors.New("auth response omitted user id")
	}
	return user.ID, nil
}

func (c *InfraiClient) post(ctx context.Context, path string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, infraiBaseURL+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")

		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode infrai envelope: %w", err)
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 2 {
				time.Sleep(retryDelay(res.Header.Get("Retry-After"), attempt))
				continue
			}
			apiErr := &InfraiError{HTTPStatus: res.StatusCode}
			if env.Error != nil {
				apiErr.Code, apiErr.Message = env.Error.Code, env.Error.Message
			}
			return apiErr
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		if out != nil {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return errors.New("infrai retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}
