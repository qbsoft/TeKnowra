// Package langflowapi authenticates only server-to-server Langflow calls.
package langflowapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	base, key string
	http      *http.Client
}
type Flow struct {
	ID     string          `json:"id"`
	UserID string          `json:"user_id"`
	Data   json.RawMessage `json:"data"`
}

func (c *Client) Version(ctx context.Context) (string, error) {
	raw, err := c.request(ctx, http.MethodGet, "/api/v1/version", nil)
	if err != nil {
		return "", err
	}
	var value struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(raw, &value) != nil || value.Version == "" {
		return "", fmt.Errorf("Langflow runtime version unavailable")
	}
	return value.Version, nil
}

func New(base, key string) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || key == "" || strings.ContainsAny(key, "\r\n") {
		return nil, fmt.Errorf("Langflow service configuration unavailable")
	}
	return &Client{base: base, key: key, http: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) request(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("Langflow request invalid")
	}
	req.Header.Set("X-API-Key", c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Langflow service unavailable")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024+1))
	if err != nil || len(raw) > 8*1024*1024 {
		return nil, fmt.Errorf("Langflow response unavailable or too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Langflow service rejected request (HTTP %d)", resp.StatusCode)
	}
	return raw, nil
}
func flowID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return fmt.Errorf("invalid flow identifier")
	}
	return nil
}
func (c *Client) GetFlow(ctx context.Context, id string) (Flow, error) {
	if err := flowID(id); err != nil {
		return Flow{}, err
	}
	raw, err := c.request(ctx, http.MethodGet, "/api/v1/flows/"+id, nil)
	if err != nil {
		return Flow{}, err
	}
	var flow Flow
	if json.Unmarshal(raw, &flow) != nil || flow.ID != id || len(flow.Data) == 0 || flow.UserID == "" {
		return Flow{}, fmt.Errorf("Langflow flow metadata invalid")
	}
	return flow, nil
}
func (c *Client) CreateFlow(ctx context.Context, name string, data json.RawMessage) (Flow, error) {
	return c.createPrivateFlow(ctx, name, "TeKnowra immutable release; do not edit", data)
}
func (c *Client) CreateDraftFlow(ctx context.Context, name string, data json.RawMessage) (Flow, error) {
	return c.createPrivateFlow(ctx, name, "TeKnowra editable draft", data)
}
func (c *Client) createPrivateFlow(ctx context.Context, name, description string, data json.RawMessage) (Flow, error) {
	body, err := json.Marshal(map[string]any{"name": name, "description": description, "access_type": "PRIVATE", "is_component": false, "data": data})
	if err != nil {
		return Flow{}, err
	}
	raw, err := c.request(ctx, http.MethodPost, "/api/v1/flows/", body)
	if err != nil {
		return Flow{}, err
	}
	var flow Flow
	if json.Unmarshal(raw, &flow) != nil || flowID(flow.ID) != nil || flow.UserID == "" || len(flow.Data) == 0 {
		return Flow{}, fmt.Errorf("Langflow created flow metadata invalid")
	}
	return flow, nil
}
func (c *Client) Run(ctx context.Context, id string, body []byte) ([]byte, error) {
	if err := flowID(id); err != nil {
		return nil, err
	}
	return c.request(ctx, http.MethodPost, "/api/v1/run/session/"+id, body)
}
