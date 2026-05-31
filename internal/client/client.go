// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

// Package client implements a minimal JMAP client for the Stalwart mail and
// collaboration server management API (Stalwart v0.16+).
//
// Stalwart removed its legacy REST management API in v0.16 and now exposes all
// configuration as JMAP objects. Management methods are served at the "/api"
// endpoint (distinct from the "/jmap" endpoint used by mail clients) and are
// negotiated through the urn:stalwart:jmap capability.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// apiPath is the HTTP path of the Stalwart management JMAP endpoint, appended
// to the configured base endpoint.
const apiPath = "/api"

// Config holds the parameters required to construct a Client.
type Config struct {
	// Endpoint is the base URL of the Stalwart server, e.g.
	// "https://mail.example.com". The client appends the management API path.
	Endpoint string

	// Token is a bearer token used for authentication. If set, it takes
	// precedence over Username/Password.
	Token string

	// Username and Password are used for HTTP Basic authentication when Token
	// is not provided.
	Username string
	Password string

	// HTTPClient is an optional custom HTTP client. If nil, a default client
	// with a sensible timeout is used.
	HTTPClient *http.Client

	// UserAgent is sent with every request.
	UserAgent string
}

// Client is a JMAP client for the Stalwart management API.
type Client struct {
	endpoint   string
	httpClient *http.Client
	userAgent  string

	token    string
	username string
	password string
}

// New constructs a Client from the given configuration.
func New(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	if cfg.Token == "" && cfg.Username == "" {
		return nil, fmt.Errorf("either a token or a username/password pair is required")
	}

	endpoint := strings.TrimRight(cfg.Endpoint, "/") + apiPath

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &Client{
		endpoint:   endpoint,
		httpClient: httpClient,
		userAgent:  cfg.UserAgent,
		token:      cfg.Token,
		username:   cfg.Username,
		password:   cfg.Password,
	}, nil
}

// HTTPError represents a non-2xx HTTP response from the server.
type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("stalwart returned HTTP %s: %s", e.Status, e.Body)
	}
	return fmt.Sprintf("stalwart returned HTTP %s", e.Status)
}

// RequestError represents a JMAP request-level error (RFC 8620, Section 3.6.1),
// returned as an RFC 7807 problem-details document when the entire request is
// rejected.
type RequestError struct {
	Type   string `json:"type"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

func (e *RequestError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("jmap request error %q: %s", e.Type, e.Detail)
	}
	return fmt.Sprintf("jmap request error %q", e.Type)
}

// MethodError represents a JMAP method-level error: a method response whose
// name is "error" (RFC 8620, Section 3.6.2).
type MethodError struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

func (e *MethodError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("jmap method error %q: %s", e.Type, e.Description)
	}
	return fmt.Sprintf("jmap method error %q", e.Type)
}

// call sends the given invocations as a single JMAP request and returns the
// decoded method responses in order. It negotiates both the JMAP core and
// Stalwart management capabilities.
func (c *Client) call(ctx context.Context, invocations ...Invocation) ([]rawInvocation, error) {
	reqBody := Request{
		Using:       []string{CapabilityCore, CapabilityStalwart},
		MethodCalls: invocations,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("encoding JMAP request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("building HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if c.userAgent != "" {
		httpReq.Header.Set("User-Agent", c.userAgent)
	}
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	} else {
		httpReq.SetBasicAuth(c.username, c.password)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("performing HTTP request: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading HTTP response: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		// A JMAP request-level error is returned as an RFC 7807 problem-details
		// document. Try to decode it for a structured error; fall back to a
		// generic HTTP error otherwise.
		var reqErr RequestError
		if err := json.Unmarshal(body, &reqErr); err == nil && reqErr.Type != "" {
			reqErr.Status = httpResp.StatusCode
			return nil, &reqErr
		}
		return nil, &HTTPError{
			StatusCode: httpResp.StatusCode,
			Status:     httpResp.Status,
			Body:       strings.TrimSpace(string(body)),
		}
	}

	var resp rawResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding JMAP response: %w (body: %s)", err, truncate(string(body), 512))
	}

	// Surface any method-level errors.
	for _, mr := range resp.MethodResponses {
		if mr.Name == "error" {
			var methodErr MethodError
			if err := json.Unmarshal(mr.Args, &methodErr); err != nil {
				return nil, fmt.Errorf("decoding JMAP method error: %w", err)
			}
			return nil, &methodErr
		}
	}

	return resp.MethodResponses, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
