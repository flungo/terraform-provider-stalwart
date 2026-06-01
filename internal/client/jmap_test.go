// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewValidation(t *testing.T) {
	cases := map[string]struct {
		cfg     Config
		wantErr bool
	}{
		"token only":            {Config{Endpoint: "https://x", Token: "t"}, false},
		"user and password":     {Config{Endpoint: "https://x", Username: "u", Password: "p"}, false},
		"no endpoint":           {Config{Token: "t"}, true},
		"no credentials":        {Config{Endpoint: "https://x"}, true},
		"token and username":    {Config{Endpoint: "https://x", Token: "t", Username: "u", Password: "p"}, true},
		"username without pass": {Config{Endpoint: "https://x", Username: "u"}, true},
	}
	for name, tc := range cases {
		_, err := New(tc.cfg)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: New() error = %v, wantErr = %v", name, err, tc.wantErr)
		}
	}
}

// TestInvocationMarshal verifies that a method call is encoded as the JMAP
// [name, arguments, callId] tuple.
func TestInvocationMarshal(t *testing.T) {
	inv := Invocation{Name: "x:Domain/get", Args: map[string]any{"ids": []string{"a"}}, CallID: "c1"}
	got, err := json.Marshal(inv)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `["x:Domain/get",{"ids":["a"]},"c1"]`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

// TestCallNegotiatesCapabilities verifies the request envelope, authentication
// header, and endpoint path, and that a get response is parsed.
func TestCallNegotiatesCapabilities(t *testing.T) {
	// The request is decoded with method calls left as raw tuples, since
	// Invocation only implements MarshalJSON (the wire encoding under test).
	type rawRequest struct {
		Using       []string        `json:"using"`
		MethodCalls [][]interface{} `json:"methodCalls"`
	}
	var gotReq rawRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != apiPath {
			t.Errorf("path = %q, want %q", r.URL.Path, apiPath)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer secret" {
			t.Errorf("Authorization = %q, want Bearer secret", auth)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &gotReq); err != nil {
			t.Errorf("decode request: %v", err)
		}
		_, _ = io.WriteString(w, `{"methodResponses":[["x:Domain/get",{"state":"1","list":[{"id":"a","name":"example.com"}],"notFound":[]},"c0"]]}`)
	}))
	defer srv.Close()

	c, err := New(Config{Endpoint: srv.URL, Token: "secret"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var dom Domain
	if err := c.GetOne(context.Background(), TypeDomain, "a", &dom); err != nil {
		t.Fatalf("GetOne: %v", err)
	}
	if dom.Name == nil || *dom.Name != "example.com" {
		t.Fatalf("domain name not parsed: %+v", dom)
	}

	wantUsing := []string{CapabilityCore, CapabilityStalwart}
	if len(gotReq.Using) != len(wantUsing) {
		t.Fatalf("using = %v, want %v", gotReq.Using, wantUsing)
	}
	for i, u := range wantUsing {
		if gotReq.Using[i] != u {
			t.Fatalf("using[%d] = %q, want %q", i, gotReq.Using[i], u)
		}
	}
	if len(gotReq.MethodCalls) != 1 || gotReq.MethodCalls[0][0] != "x:Domain/get" {
		t.Fatalf("methodCalls = %v, want a single x:Domain/get call", gotReq.MethodCalls)
	}
}

// TestMethodError verifies that a method-level "error" response is surfaced as
// a *MethodError.
func TestMethodError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"methodResponses":[["error",{"type":"unknownMethod","description":"nope"},"c0"]]}`)
	}))
	defer srv.Close()

	c, _ := New(Config{Endpoint: srv.URL, Token: "secret"})
	_, err := c.Get(context.Background(), TypeDomain, []string{"a"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	me, ok := err.(*MethodError)
	if !ok {
		t.Fatalf("error type = %T, want *MethodError", err)
	}
	if me.Type != "unknownMethod" {
		t.Fatalf("error type = %q, want unknownMethod", me.Type)
	}
}

// TestHTTPError verifies that a non-2xx response becomes an *HTTPError.
func TestHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, "unauthorized")
	}))
	defer srv.Close()

	c, _ := New(Config{Endpoint: srv.URL, Token: "secret"})
	_, err := c.Get(context.Background(), TypeDomain, []string{"a"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if he, ok := err.(*HTTPError); !ok || he.StatusCode != http.StatusUnauthorized {
		t.Fatalf("error = %v (%T), want *HTTPError 401", err, err)
	}
}

func TestIsID(t *testing.T) {
	cases := map[string]bool{
		"itxnfyrwaaaa":   true,  // real server id (base32, lowercase)
		"a":              true,  // id 0 encodes as "a"
		"c":              true,  // short ids are valid
		"b792013":        true,  // includes the digit subset of the alphabet
		"example.test":   false, // domain name (contains '.')
		"alice@dom.test": false, // email (contains '@')
		"support role":   false, // role description (contains space)
		"ABCDEF":         false, // uppercase is not in the alphabet
		"id-with-dash":   false, // '-' is not in the alphabet
		"abc456":         false, // digits 4,5,6 are not in the alphabet
		"":               false, // empty
		"aaaaaaaaaaaaaa": false, // 14 chars exceeds a base32 u64
	}
	for in, want := range cases {
		if got := IsID(in); got != want {
			t.Errorf("IsID(%q) = %v, want %v", in, got, want)
		}
	}
}
