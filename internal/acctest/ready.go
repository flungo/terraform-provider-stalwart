// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package acctest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// readyTimeout bounds how long Start waits for the management API to accept an
// authenticated JMAP request. First boot initialises the RocksDB store, which
// can take a little while on a cold CI runner.
const readyTimeout = 120 * time.Second

// waitReady polls the container's management API until it answers an
// authenticated JMAP request, or the timeout elapses. It uses the same client
// the provider uses, so a successful probe also validates auth and the JMAP
// endpoint path end-to-end.
//
// On timeout it returns a diagnosis assembled from low-level HTTP probes of the
// candidate endpoints, so the failure message explains *why* the API never
// answered (wrong path, auth rejected, still booting, ...) even when the raw CI
// logs are not retrievable.
func waitReady(ctx context.Context, c *Container) error {
	cl, err := client.New(client.Config{
		Endpoint: c.endpoint,
		Username: adminUser,
		Password: adminPassword,
	})
	if err != nil {
		return err
	}

	deadline := time.Now().Add(readyTimeout)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var lastErr error
	for {
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, lastErr = cl.Query(probeCtx, client.TypeDomain, map[string]any{})
		cancel()
		if lastErr == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for the JMAP management API: %w\n%s",
				readyTimeout, lastErr, diagnose(ctx, c.endpoint))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// diagnose probes a set of candidate endpoints with the recovery-admin
// credentials and returns a human-readable summary of each response. It is used
// to explain a readiness timeout without access to the container's own logs.
func diagnose(ctx context.Context, base string) string {
	type probe struct {
		method, path, body string
	}
	jmapBody := `{"using":["urn:ietf:params:jmap:core","urn:stalwart:jmap"],` +
		`"methodCalls":[["x:Domain/query",{"filter":{}},"c0"]]}`
	probes := []probe{
		{http.MethodGet, "/healthz/live", ""},
		{http.MethodGet, "/healthz/ready", ""},
		{http.MethodPost, "/api", jmapBody},
		{http.MethodPost, "/jmap", jmapBody},
		{http.MethodGet, "/api/schema", ""},
	}

	var b strings.Builder
	b.WriteString("endpoint diagnostics:\n")
	hc := &http.Client{Timeout: 5 * time.Second}
	for _, p := range probes {
		url := strings.TrimRight(base, "/") + p.path
		var body io.Reader
		if p.body != "" {
			body = strings.NewReader(p.body)
		}
		req, err := http.NewRequestWithContext(ctx, p.method, url, body)
		if err != nil {
			fmt.Fprintf(&b, "  %s %s -> request error: %v\n", p.method, p.path, err)
			continue
		}
		req.SetBasicAuth(adminUser, adminPassword)
		if p.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := hc.Do(req)
		if err != nil {
			fmt.Fprintf(&b, "  %s %s -> %v\n", p.method, p.path, err)
			continue
		}
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		_ = resp.Body.Close()
		fmt.Fprintf(&b, "  %s %s -> %d %s\n", p.method, p.path, resp.StatusCode,
			strings.TrimSpace(string(respBody)))
	}
	return b.String()
}
