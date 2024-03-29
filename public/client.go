// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package public

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/mattermost/calls-offloader/public/job"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
)

type Client struct {
	cfg        *ClientConfig
	httpClient *http.Client
	dialFn     DialContextFn
	authToken  string
}

type ClientConfig struct {
	httpURL string

	ClientID string
	AuthKey  string
	URL      string
}

func (c *ClientConfig) Parse() error {
	if c.URL == "" {
		return fmt.Errorf("invalid URL value: should not be empty")
	}

	u, err := url.Parse(c.URL)
	if err != nil {
		return fmt.Errorf("failed to parse url: %w", err)
	}

	if u.Host == "" {
		return fmt.Errorf("invalid url host: should not be empty")
	}

	switch u.Scheme {
	case "http", "https":
		c.httpURL = c.URL
	default:
		return fmt.Errorf("invalid url scheme: %q is not valid", u.Scheme)
	}

	return nil
}

type ClientOption func(c *Client) error
type DialContextFn func(ctx context.Context, network, addr string) (net.Conn, error)

// WithDialFunc lets the caller set an optional dialing function to setup the
// HTTP/WebSocket connections used by the client.
func WithDialFunc(dialFn DialContextFn) ClientOption {
	return func(c *Client) error {
		c.dialFn = dialFn
		return nil
	}
}

func NewClient(cfg ClientConfig, opts ...ClientOption) (*Client, error) {
	var c Client

	if err := cfg.Parse(); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	c.cfg = &cfg

	for _, opt := range opts {
		if err := opt(&c); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	dialFn := (&net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}).DialContext

	if c.dialFn != nil {
		dialFn = c.dialFn
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialFn,
		MaxConnsPerHost:       100,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		ResponseHeaderTimeout: 2 * time.Minute,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   1 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	c.httpClient = &http.Client{Transport: transport}

	return &c, nil
}

func (c *Client) Register(clientID string, authKey string) error {
	if c.httpClient == nil {
		return fmt.Errorf("http client is not initialized")
	}

	reqData := map[string]string{
		"clientID": clientID,
		"authKey":  authKey,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(reqData); err != nil {
		return fmt.Errorf("failed to encode body: %w", err)
	}

	req, err := http.NewRequest("POST", c.cfg.httpURL+"/register", &buf)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.SetBasicAuth(c.cfg.ClientID, c.cfg.AuthKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		return nil
	} else if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}

	respData := map[string]string{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return fmt.Errorf("decoding http response failed: %w", err)
	}

	if errMsg := respData["error"]; errMsg != "" {
		return fmt.Errorf("request failed: %s", errMsg)
	}
	return fmt.Errorf("request failed with status %s", resp.Status)
}

func (c *Client) Unregister(clientID string) error {
	if c.httpClient == nil {
		return fmt.Errorf("http client is not initialized")
	}

	reqData := map[string]string{
		"clientID": clientID,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(reqData); err != nil {
		return fmt.Errorf("failed to encode body: %w", err)
	}

	req, err := http.NewRequest("POST", c.cfg.httpURL+"/unregister", &buf)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.SetBasicAuth(c.cfg.ClientID, c.cfg.AuthKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	} else if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}

	respData := map[string]string{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return fmt.Errorf("decoding http response failed: %w", err)
	}

	if errMsg := respData["error"]; errMsg != "" {
		return fmt.Errorf("request failed: %s", errMsg)
	}
	return fmt.Errorf("request failed with status %s", resp.Status)
}

func (c *Client) Login(clientID string, authKey string) error {
	if c.httpClient == nil {
		return fmt.Errorf("http client is not initialized")
	}

	reqData := map[string]string{
		"clientID": clientID,
		"authKey":  authKey,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(reqData); err != nil {
		return fmt.Errorf("failed to encode body: %w", err)
	}

	req, err := http.NewRequest("POST", c.cfg.httpURL+"/login", &buf)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respData := map[string]string{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return fmt.Errorf("decoding http response failed: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		c.authToken = respData["bearerToken"]
		return nil
	} else if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}

	if errMsg := respData["error"]; errMsg != "" {
		return fmt.Errorf("request failed: %s", errMsg)
	}

	return fmt.Errorf("request failed with status %s", resp.Status)
}

func (c *Client) CreateJob(cfg job.Config) (job.Job, error) {
	if c.httpClient == nil {
		return job.Job{}, fmt.Errorf("http client is not initialized")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(cfg); err != nil {
		return job.Job{}, fmt.Errorf("failed to encode body: %w", err)
	}

	req, err := http.NewRequest("POST", c.cfg.httpURL+"/jobs", &buf)
	if err != nil {
		return job.Job{}, fmt.Errorf("failed to build request: %w", err)
	}
	req.SetBasicAuth(c.cfg.ClientID, c.cfg.AuthKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return job.Job{}, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var j job.Job
		if err := json.NewDecoder(resp.Body).Decode(&j); err != nil {
			return job.Job{}, fmt.Errorf("decoding http response failed: %w", err)
		}
		return j, nil
	} else if resp.StatusCode == http.StatusUnauthorized {
		return job.Job{}, ErrUnauthorized
	}

	respData := map[string]any{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return job.Job{}, fmt.Errorf("decoding http response failed: %w", err)
	}

	if errMsg, _ := respData["error"].(string); errMsg != "" {
		return job.Job{}, fmt.Errorf("request failed: %s", errMsg)
	}

	return job.Job{}, fmt.Errorf("request failed with status %s", resp.Status)
}

func (c *Client) GetJob(jobID string) (job.Job, error) {
	if c.httpClient == nil {
		return job.Job{}, fmt.Errorf("http client is not initialized")
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/jobs/%s", c.cfg.httpURL, jobID), nil)
	if err != nil {
		return job.Job{}, fmt.Errorf("failed to build request: %w", err)
	}
	req.SetBasicAuth(c.cfg.ClientID, c.cfg.AuthKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return job.Job{}, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var j job.Job
		if err := json.NewDecoder(resp.Body).Decode(&j); err != nil {
			return job.Job{}, fmt.Errorf("decoding http response failed: %w", err)
		}

		return j, nil
	} else if resp.StatusCode == http.StatusUnauthorized {
		return job.Job{}, ErrUnauthorized
	}

	respData := map[string]any{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return job.Job{}, fmt.Errorf("decoding http response failed: %w", err)
	}
	if errMsg, _ := respData["error"].(string); errMsg != "" {
		return job.Job{}, fmt.Errorf("request failed: %s", errMsg)
	}
	return job.Job{}, fmt.Errorf("request failed with status %s", resp.Status)
}

func (c *Client) GetJobLogs(jobID string) ([]byte, error) {
	if c.httpClient == nil {
		return nil, fmt.Errorf("http client is not initialized")
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/jobs/%s/logs", c.cfg.httpURL, jobID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	req.SetBasicAuth(c.cfg.ClientID, c.cfg.AuthKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		return data, nil
	} else if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}

	return nil, fmt.Errorf("request failed with status %s", resp.Status)
}

func (c *Client) Init(cfg job.ServiceConfig) error {
	if c.httpClient == nil {
		return fmt.Errorf("http client is not initialized")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&cfg); err != nil {
		return fmt.Errorf("failed to encode body: %w", err)
	}

	req, err := http.NewRequest("POST", c.cfg.httpURL+"/jobs/init", &buf)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.SetBasicAuth(c.cfg.ClientID, c.cfg.AuthKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	} else if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}

	respData := map[string]any{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return fmt.Errorf("decoding http response failed: %w", err)
	}
	if errMsg, _ := respData["error"].(string); errMsg != "" {
		return fmt.Errorf("request failed: %s", errMsg)
	}
	return fmt.Errorf("request failed with status %s", resp.Status)
}

func (c *Client) Close() error {
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
	return nil
}

func (c *Client) GetVersionInfo() (VersionInfo, error) {
	if c.httpClient == nil {
		return VersionInfo{}, fmt.Errorf("http client is not initialized")
	}

	req, err := http.NewRequest("GET", c.cfg.httpURL+"/version", nil)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("failed to build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	var info VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return VersionInfo{}, fmt.Errorf("decoding http response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return VersionInfo{}, fmt.Errorf("request failed with status %s", resp.Status)
	}

	return info, nil
}

func (c *Client) AuthToken() string {
	return c.authToken
}

func (c *Client) URL() string {
	return c.cfg.httpURL
}
