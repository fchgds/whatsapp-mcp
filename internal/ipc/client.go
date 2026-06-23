package ipc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	var st Status
	err := c.do(ctx, http.MethodGet, "/status", nil, &st)
	return st, err
}

func (c *Client) Download(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
	var res DownloadResult
	err := c.do(ctx, http.MethodPost, "/download", req, &res)
	return res, err
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("X-Token", c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon respondió %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
