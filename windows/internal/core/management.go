package core

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
	"time"
)

// Management is a thin client for the engine's /v8/management API on loopback.
type Management struct {
	Port     int
	Password string
}

// The client never goes through a system proxy: the engine is on loopback.
var managementHTTP = &http.Client{
	Timeout:   15 * time.Second,
	Transport: &http.Transport{Proxy: nil, MaxIdleConnsPerHost: 4, IdleConnTimeout: 30 * time.Second},
}

// Long polls (access requests) wait up to a minute, so they get their own
// client without the 15s timeout; the request context bounds them instead.
var longPollHTTP = &http.Client{
	Transport: &http.Transport{Proxy: nil, MaxIdleConnsPerHost: 2, IdleConnTimeout: 90 * time.Second},
}

// APIError is a non-2xx answer from the engine.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("Engine returned %d: %s", e.Status, e.Message) }

func isNotFound(err error) bool {
	var api *APIError
	return errors.As(err, &api) && api.Status == http.StatusNotFound
}

func (m Management) Get(ctx context.Context, path string, query url.Values, out any) error {
	data, err := m.send(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unexpected response from %s: %w", path, err)
	}
	return nil
}

// GetOptional maps 404 (path absent from config) to found=false.
func (m Management) GetOptional(ctx context.Context, path string, out any) (bool, error) {
	err := m.Get(ctx, path, nil, out)
	if isNotFound(err) {
		return false, nil
	}
	return err == nil, err
}

// LongGet is Get for endpoints that hold the request open (?wait=).
func (m Management) LongGet(ctx context.Context, path string, query url.Values, out any) error {
	data, err := m.sendWith(ctx, longPollHTTP, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unexpected response from %s: %w", path, err)
	}
	return nil
}

// Post sends a body-less POST and decodes the answer into out (if non-nil).
func (m Management) Post(ctx context.Context, path string, out any) error {
	data, err := m.send(ctx, http.MethodPost, path, nil, nil)
	if err != nil || out == nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func (m Management) Put(ctx context.Context, path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	_, err = m.send(ctx, http.MethodPut, path, nil, data)
	return err
}

func (m Management) Delete(ctx context.Context, path string, query url.Values) error {
	_, err := m.send(ctx, http.MethodDelete, path, query, nil)
	return err
}

func (m Management) Healthy(ctx context.Context) bool {
	_, err := m.send(ctx, http.MethodGet, "/v8/management/config/server", nil, nil)
	return err == nil
}

func (m Management) send(ctx context.Context, method, path string, query url.Values, body []byte) ([]byte, error) {
	return m.sendWith(ctx, managementHTTP, method, path, query, body)
}

func (m Management) sendWith(ctx context.Context, client *http.Client, method, path string, query url.Values, body []byte) ([]byte, error) {
	u := fmt.Sprintf("http://127.0.0.1:%d%s", m.Port, path)
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+m.Password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var eb struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		msg := strings.TrimSpace(string(data))
		if json.Unmarshal(data, &eb) == nil {
			if eb.Message != "" {
				msg = eb.Message
			} else if eb.Error != "" {
				msg = eb.Error
			}
		}
		return nil, &APIError{Status: resp.StatusCode, Message: msg}
	}
	return data, nil
}

// Response models (subset of what the engine returns).

type authURLResponse struct {
	URL   string `json:"url"`
	State string `json:"state"`
}

type authStatusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

type credentialList struct {
	Files []Credential `json:"files"`
}

// Credential is one signed-in account file.
type Credential struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Provider      string `json:"provider"`
	Email         string `json:"email,omitempty"`
	Label         string `json:"label,omitempty"`
	Status        string `json:"status,omitempty"`
	StatusMessage string `json:"status_message,omitempty"`
	Disabled      bool   `json:"disabled,omitempty"`
	Unavailable   bool   `json:"unavailable,omitempty"`
	IDToken       *struct {
		PlanType string `json:"plan_type"`
	} `json:"id_token,omitempty"`
}

func (c Credential) DisplayName() string {
	switch {
	case c.Email != "":
		return c.Email
	case c.Label != "":
		return c.Label
	}
	return c.Name
}

func (c Credential) Plan() string {
	if c.IDToken == nil || c.IDToken.PlanType == "" {
		return ""
	}
	p := c.IDToken.PlanType
	return strings.ToUpper(p[:1]) + p[1:]
}

func (c Credential) Healthy() bool { return !c.Disabled && !c.Unavailable }

type modelDefinitionList struct {
	Models []ModelDefinition `json:"models"`
}

// ModelDefinition is one model in a provider's catalog.
type ModelDefinition struct {
	ID            string `json:"id"`
	DisplayName   string `json:"display_name,omitempty"`
	ContextLength int    `json:"context_length,omitempty"`
}

// ModelAlias exposes Name under an extra client-visible Alias.
type ModelAlias struct {
	Name  string `json:"name"`
	Alias string `json:"alias"`
	Fork  bool   `json:"fork,omitempty"`
}

type claudeUpstream struct {
	DisableCloakMode         *bool `json:"disable-claude-cloak-mode"`
	DisableCloakingModelList *bool `json:"disable-cloaking-model-list"`
}

type codexUpstream struct {
	DisableCloaking *bool `json:"disable-codex-cloaking"`
}

type usageSummary struct {
	Total Usage `json:"total"`
}

// Usage is the request/token count for a period.
type Usage struct {
	Requests int `json:"requests"`
	Failures int `json:"failures"`
	Tokens   struct {
		Total int `json:"total"`
	} `json:"tokens"`
	CostUSD *float64 `json:"cost_usd,omitempty"`
}
