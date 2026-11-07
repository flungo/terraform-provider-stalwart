// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// schemaDoc returns a minimal registry schema document whose Domain object
// has the given enable-flag property, gzip-compressed as Stalwart serves it.
func schemaDoc(t *testing.T, enableProp string) []byte {
	t.Helper()
	doc := map[string]any{
		"fields": map[string]any{
			"x:Domain": map[string]any{
				"properties": map[string]any{
					"name":     map[string]any{"type": map[string]any{"type": "string"}},
					enableProp: map[string]any{"type": map[string]any{"type": "boolean"}},
				},
			},
		},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write(raw)
	_ = gz.Close()
	return buf.Bytes()
}

// fakeServer emulates the two endpoints the client uses. /api/schema redirects
// to a hashed path that serves the gzip document with Content-Encoding: gzip
// (crates/http/src/api/mod.rs). /jmap records the last request body and
// answers with jmapResponse.
type fakeServer struct {
	*httptest.Server
	schemaHits   atomic.Int32
	lastJMAP     []byte
	jmapResponse string
}

func newFakeServer(t *testing.T, enableProp string) *fakeServer {
	t.Helper()
	fs := &fakeServer{}
	doc := schemaDoc(t, enableProp)
	fs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "admin" || pass != "pw" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/schema":
			http.Redirect(w, r, "/api/schema/abc123", http.StatusTemporaryRedirect)
		case "/api/schema/abc123":
			fs.schemaHits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Encoding", "gzip")
			_, _ = w.Write(doc)
		case "/jmap":
			fs.lastJMAP, _ = io.ReadAll(r.Body)
			_, _ = io.WriteString(w, fs.jmapResponse)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fs.Close)
	return fs
}

func newTestClient(t *testing.T, url string, v ServerVersion) *Client {
	t.Helper()
	c, err := New(Config{Endpoint: url, Username: "admin", Password: "pw", ServerVersion: v})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestServerVersionDetection(t *testing.T) {
	for prop, want := range map[string]ServerVersion{"isEnabled": Version016, "enable": Version1} {
		t.Run(prop, func(t *testing.T) {
			fs := newFakeServer(t, prop)
			c := newTestClient(t, fs.URL, VersionUnknown)
			for range 2 {
				got, err := c.ServerVersion(context.Background())
				if err != nil {
					t.Fatalf("ServerVersion: %v", err)
				}
				if got != want {
					t.Fatalf("ServerVersion = %v, want %v", got, want)
				}
			}
			if n := fs.schemaHits.Load(); n != 1 {
				t.Errorf("schema fetched %d times, want 1 (cached)", n)
			}
		})
	}
}

// TestServerVersionCompressionDisabled covers a caller-supplied HTTP client
// that does not decompress: the raw gzip body must still be recognised.
func TestServerVersionCompressionDisabled(t *testing.T) {
	fs := newFakeServer(t, "enable")
	c, err := New(Config{
		Endpoint: fs.URL, Username: "admin", Password: "pw",
		HTTPClient: &http.Client{Transport: &http.Transport{DisableCompression: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.ServerVersion(context.Background())
	if err != nil || got != Version1 {
		t.Fatalf("ServerVersion = %v, %v; want 1, nil", got, err)
	}
}

func TestServerVersionOverrideSkipsDetection(t *testing.T) {
	fs := newFakeServer(t, "enable")
	c := newTestClient(t, fs.URL, Version016)
	got, err := c.ServerVersion(context.Background())
	if err != nil || got != Version016 {
		t.Fatalf("ServerVersion = %v, %v; want 0.16, nil", got, err)
	}
	if n := fs.schemaHits.Load(); n != 0 {
		t.Errorf("schema fetched %d times, want 0", n)
	}
}

func TestServerVersionErrors(t *testing.T) {
	t.Run("ambiguous schema", func(t *testing.T) {
		fs := newFakeServer(t, "somethingElse")
		_, err := newTestClient(t, fs.URL, VersionUnknown).ServerVersion(context.Background())
		if err == nil || !strings.Contains(err.Error(), "server_version") {
			t.Fatalf("err = %v, want a hint about server_version", err)
		}
	})
	t.Run("unauthorized", func(t *testing.T) {
		fs := newFakeServer(t, "enable")
		c, _ := New(Config{Endpoint: fs.URL, Username: "admin", Password: "wrong"})
		if _, err := c.ServerVersion(context.Background()); err == nil {
			t.Fatal("expected an error")
		}
	})
}

func TestParseServerVersion(t *testing.T) {
	for in, want := range map[string]ServerVersion{
		"0.16": Version016, "v0.16": Version016, "1": Version1, "1.0": Version1, "v1": Version1,
	} {
		got, err := ParseServerVersion(in)
		if err != nil || got != want {
			t.Errorf("ParseServerVersion(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "0.15", "2"} {
		if _, err := ParseServerVersion(in); err == nil {
			t.Errorf("ParseServerVersion(%q): expected an error", in)
		}
	}
}

func TestRenameKeys(t *testing.T) {
	table := map[string]string{"enable": "isEnabled"}
	got, err := renameKeys(json.RawMessage(`{"enable":true,"enable/x":1,"name":"a","enableX":2}`), table)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(got, &m)
	for _, k := range []string{"isEnabled", "isEnabled/x", "name", "enableX"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q in %s", k, got)
		}
	}
	if _, ok := m["enable"]; ok {
		t.Errorf("key enable was not renamed: %s", got)
	}
}

// TestWireTranslation checks that a v0.16 server receives and returns the
// v0.16 property names while callers only ever see the v1 names, and that a
// v1 server's traffic is passed through untouched.
func TestWireTranslation(t *testing.T) {
	ctx := context.Background()
	enable := true

	t.Run("v0.16", func(t *testing.T) {
		fs := newFakeServer(t, "isEnabled")
		c := newTestClient(t, fs.URL, VersionUnknown)

		fs.jmapResponse = `{"methodResponses":[["x:Domain/set",{"created":{"new":{"id":"b"}}},"c0"]]}`
		if _, err := c.Create(ctx, TypeDomain, Domain{Enable: &enable}, nil); err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(fs.lastJMAP, []byte(`"isEnabled":true`)) || bytes.Contains(fs.lastJMAP, []byte(`"enable"`)) {
			t.Errorf("create payload not translated: %s", fs.lastJMAP)
		}

		fs.jmapResponse = `{"methodResponses":[["x:Domain/set",{"updated":{"b":null}},"c0"]]}`
		if err := c.Update(ctx, TypeDomain, "b", map[string]any{"enable": false}); err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(fs.lastJMAP, []byte(`"isEnabled":false`)) {
			t.Errorf("update patch not translated: %s", fs.lastJMAP)
		}

		fs.jmapResponse = `{"methodResponses":[["x:Domain/get",{"list":[{"id":"b","isEnabled":true}]},"c0"]]}`
		var dom Domain
		if err := c.GetOne(ctx, TypeDomain, "b", &dom); err != nil {
			t.Fatal(err)
		}
		if dom.Enable == nil || !*dom.Enable {
			t.Errorf("get result not translated: Enable = %v", dom.Enable)
		}

		// Objects without renames are never re-encoded.
		fs.jmapResponse = `{"methodResponses":[["x:Role/set",{"created":{"new":{"id":"r"}}},"c0"]]}`
		if _, err := c.Create(ctx, TypeRole, Role{}, nil); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("v1", func(t *testing.T) {
		fs := newFakeServer(t, "enable")
		c := newTestClient(t, fs.URL, VersionUnknown)

		fs.jmapResponse = `{"methodResponses":[["x:Domain/set",{"created":{"new":{"id":"b"}}},"c0"]]}`
		if _, err := c.Create(ctx, TypeDomain, Domain{Enable: &enable}, nil); err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(fs.lastJMAP, []byte(`"enable":true`)) {
			t.Errorf("create payload altered for v1: %s", fs.lastJMAP)
		}

		fs.jmapResponse = `{"methodResponses":[["x:Domain/get",{"list":[{"id":"b","enable":true}]},"c0"]]}`
		var dom Domain
		if err := c.GetOne(ctx, TypeDomain, "b", &dom); err != nil {
			t.Fatal(err)
		}
		if dom.Enable == nil || !*dom.Enable {
			t.Errorf("Enable = %v, want true", dom.Enable)
		}
	})
}
