// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ServerVersion identifies which generation of the Stalwart management schema
// the server speaks. Stalwart v0.16 and v1.0 expose the same JMAP management
// API, but v1.0 renamed and removed a number of object properties (see
// docs/decisions/004-dual-server-version-support.md). Within a generation the
// properties this provider uses are stable, so the generation is all the
// provider needs to know.
type ServerVersion int

const (
	// VersionUnknown means the version has not been detected yet.
	VersionUnknown ServerVersion = iota
	// Version016 is the Stalwart v0.16.x management schema.
	Version016
	// Version1 is the Stalwart v1.x management schema.
	Version1
)

// String renders the version in the form the provider's `server_version`
// attribute accepts.
func (v ServerVersion) String() string {
	switch v {
	case Version016:
		return "0.16"
	case Version1:
		return "1"
	default:
		return "unknown"
	}
}

// ParseServerVersion parses the `server_version` provider setting. It accepts
// "0.16" and "1" (and the forms "v0.16", "1.0", "v1" for convenience).
func ParseServerVersion(s string) (ServerVersion, error) {
	switch strings.TrimPrefix(strings.TrimSpace(s), "v") {
	case "0.16":
		return Version016, nil
	case "1", "1.0":
		return Version1, nil
	default:
		return VersionUnknown, fmt.Errorf("unsupported Stalwart server version %q: expected \"0.16\" or \"1\"", s)
	}
}

// schemaPath is the HTTP API path that serves the server's own registry
// schema (the document the WebUI renders its forms from). It exists in every
// v0.16 and v1 release and needs only an authenticated request. It redirects
// to a content-addressed /api/schema/<sha256>, which the HTTP client follows.
const schemaPath = "/api/schema"

// versionProbe names the property whose name tells the two schema generations
// apart: Domain.isEnabled (v0.16) was renamed to Domain.enable (v1).
const (
	versionProbeObject = "x:Domain"
	versionProbeV016   = "isEnabled"
	versionProbeV1     = "enable"
)

// ServerVersion returns the schema generation of the server, detecting it on
// first use. A version set in Config is returned without contacting the
// server. A failed detection is not cached, so a transient error does not
// poison the client.
func (c *Client) ServerVersion(ctx context.Context) (ServerVersion, error) {
	c.versionMu.Lock()
	defer c.versionMu.Unlock()
	if c.version != VersionUnknown {
		return c.version, nil
	}
	v, err := c.detectVersion(ctx)
	if err != nil {
		return VersionUnknown, err
	}
	c.version = v
	return v, nil
}

// detectVersion fetches the server's registry schema and classifies it by the
// name of the Domain enable flag.
func (c *Client) detectVersion(ctx context.Context) (ServerVersion, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+schemaPath, nil)
	if err != nil {
		return VersionUnknown, fmt.Errorf("building schema request: %w", err)
	}
	c.authorize(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return VersionUnknown, fmt.Errorf("detecting Stalwart server version: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return VersionUnknown, fmt.Errorf("detecting Stalwart server version: GET %s: %w; "+
			"set the provider's `server_version` to skip detection",
			schemaPath, &HTTPError{StatusCode: resp.StatusCode, Status: resp.Status, Body: strings.TrimSpace(string(body))})
	}

	// The server always serves this document gzip-compressed. Go's transport
	// decompresses it transparently when it negotiated compression itself; a
	// caller-supplied client with compression disabled hands back the raw gzip
	// stream, so sniff the magic bytes rather than trust headers.
	body := bufio.NewReader(resp.Body)
	var r io.Reader = body
	if magic, err := body.Peek(2); err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(body)
		if err != nil {
			return VersionUnknown, fmt.Errorf("detecting Stalwart server version: %w", err)
		}
		defer func() { _ = gz.Close() }()
		r = gz
	}
	return classifySchema(r)
}

// classifySchema decodes just enough of the registry schema document to read
// the properties of the probe object.
func classifySchema(r io.Reader) (ServerVersion, error) {
	var doc struct {
		Fields map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"fields"`
	}
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return VersionUnknown, fmt.Errorf("detecting Stalwart server version: decoding %s: %w", schemaPath, err)
	}
	props := doc.Fields[versionProbeObject].Properties
	_, hasV1 := props[versionProbeV1]
	_, hasV016 := props[versionProbeV016]
	switch {
	case hasV1 && !hasV016:
		return Version1, nil
	case hasV016 && !hasV1:
		return Version016, nil
	default:
		return VersionUnknown, fmt.Errorf("detecting Stalwart server version: the server's schema has neither "+
			"%s.%s (v0.16) nor %s.%s (v1); set the provider's `server_version` to skip detection",
			versionProbeObject, versionProbeV016, versionProbeObject, versionProbeV1)
	}
}

// v016Renames maps, per object type, the v1 name of a property to its v0.16
// name. The client models every object with the v1 names; on a v0.16 server it
// renames these keys in outgoing set payloads and incoming get results.
//
// Only properties this provider sends or reads are listed. Source:
// stalwart-migrate SCHEMA_CHANGES.md section 3.1 at daf90f0, checked against
// the schema snapshots mapping/schema/v0.16.27.json.gz and v1.json.gz.
var v016Renames = map[string]map[string]string{
	TypeDomain:          {"enable": "isEnabled"},
	TypeDnsServer:       {"pollInterval": "pollingInterval"},
	TypeNetworkListener: {"implicitTls": "tlsImplicit"},
	TypeDirectory:       {"filterRecipient": "filterMailbox"},
}

// renameKeys returns a copy of obj (a JSON object) with its top-level keys
// renamed by table. A JMAP patch path ("a/b") is renamed by its first segment.
// Values that are not JSON objects are returned unchanged.
func renameKeys(raw json.RawMessage, table map[string]string) (json.RawMessage, error) {
	if len(table) == 0 {
		return raw, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return raw, nil //nolint:nilerr // not an object: nothing to rename
	}
	out := make(map[string]json.RawMessage, len(obj))
	for k, v := range obj {
		head, rest, hasRest := strings.Cut(k, "/")
		if to, ok := table[head]; ok {
			k = to
			if hasRest {
				k += "/" + rest
			}
		}
		out[k] = v
	}
	return json.Marshal(out)
}

// invert returns the reverse of a rename table.
func invert(table map[string]string) map[string]string {
	out := make(map[string]string, len(table))
	for k, v := range table {
		out[v] = k
	}
	return out
}

// toWire converts an object or patch modelled with v1 property names into the
// server's dialect.
func (c *Client) toWire(ctx context.Context, objType string, obj any) (any, error) {
	table := v016Renames[objType]
	if table == nil {
		return obj, nil
	}
	v, err := c.ServerVersion(ctx)
	if err != nil {
		return nil, err
	}
	if v != Version016 {
		return obj, nil
	}
	raw, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("encoding %s: %w", objType, err)
	}
	return renameKeys(raw, table)
}

// fromWire converts an object returned by the server into v1 property names.
func (c *Client) fromWire(ctx context.Context, objType string, raw json.RawMessage) (json.RawMessage, error) {
	table := v016Renames[objType]
	if table == nil {
		return raw, nil
	}
	v, err := c.ServerVersion(ctx)
	if err != nil {
		return nil, err
	}
	if v != Version016 {
		return raw, nil
	}
	return renameKeys(raw, invert(table))
}
