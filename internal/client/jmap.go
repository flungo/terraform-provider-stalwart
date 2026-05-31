// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"encoding/json"
	"fmt"
)

// Capability URNs used by the Stalwart management JMAP API.
//
// Every management/configuration object is exposed via the urn:stalwart:jmap
// capability and requires the standard JMAP core capability to be negotiated
// as well. See https://stalw.art/docs/ref/ for details.
const (
	CapabilityCore     = "urn:ietf:params:jmap:core"
	CapabilityStalwart = "urn:stalwart:jmap"
)

// MethodPrefix is prepended to every management object type on the wire. The
// Stalwart schema reference documents management methods such as "x:Domain/get"
// and "x:Account/set"; the "x:" prefix denotes a vendor (Stalwart) extension
// type and is required on the wire (it is only omitted by the CLI for display).
const MethodPrefix = "x:"

// Request is the JMAP request envelope as defined by RFC 8620, Section 3.3.
type Request struct {
	Using       []string     `json:"using"`
	MethodCalls []Invocation `json:"methodCalls"`
}

// Invocation is a single JMAP method call. On the wire it is encoded as a
// three element array of [name, arguments, callId] (RFC 8620, Section 3.2).
type Invocation struct {
	Name   string
	Args   any
	CallID string
}

// MarshalJSON encodes the invocation as the [name, arguments, callId] tuple
// expected by JMAP.
func (i Invocation) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{i.Name, i.Args, i.CallID})
}

// rawResponse is the JMAP response envelope (RFC 8620, Section 3.4).
type rawResponse struct {
	MethodResponses []rawInvocation `json:"methodResponses"`
	SessionState    string          `json:"sessionState"`
}

// rawInvocation is a method response with its arguments left as raw JSON so
// that callers can decode them into the appropriate concrete type.
type rawInvocation struct {
	Name   string
	Args   json.RawMessage
	CallID string
}

// UnmarshalJSON decodes the [name, arguments, callId] tuple, keeping the
// arguments as raw JSON.
func (r *rawInvocation) UnmarshalJSON(data []byte) error {
	var tuple [3]json.RawMessage
	if err := json.Unmarshal(data, &tuple); err != nil {
		return fmt.Errorf("decoding method response tuple: %w", err)
	}
	if err := json.Unmarshal(tuple[0], &r.Name); err != nil {
		return fmt.Errorf("decoding method response name: %w", err)
	}
	r.Args = tuple[1]
	if err := json.Unmarshal(tuple[2], &r.CallID); err != nil {
		return fmt.Errorf("decoding method response call id: %w", err)
	}
	return nil
}

// GetResponse is the result of a standard "Foo/get" method (RFC 8620, 5.1).
type GetResponse struct {
	State    string            `json:"state"`
	List     []json.RawMessage `json:"list"`
	NotFound []string          `json:"notFound"`
}

// SetResponse is the result of a standard "Foo/set" method (RFC 8620, 5.3).
type SetResponse struct {
	OldState     string                     `json:"oldState"`
	NewState     string                     `json:"newState"`
	Created      map[string]json.RawMessage `json:"created"`
	Updated      map[string]json.RawMessage `json:"updated"`
	Destroyed    []string                   `json:"destroyed"`
	NotCreated   map[string]SetError        `json:"notCreated"`
	NotUpdated   map[string]SetError        `json:"notUpdated"`
	NotDestroyed map[string]SetError        `json:"notDestroyed"`
}

// QueryResponse is the result of a standard "Foo/query" method (RFC 8620, 5.5).
type QueryResponse struct {
	QueryState string   `json:"queryState"`
	IDs        []string `json:"ids"`
	Total      int      `json:"total"`
}

// SetError describes why a single create/update/destroy operation failed
// within an otherwise successful "Foo/set" call (RFC 8620, Section 5.3).
type SetError struct {
	Type        string   `json:"type"`
	Description *string  `json:"description"`
	Properties  []string `json:"properties"`
}

func (e SetError) Error() string {
	msg := e.Type
	if e.Description != nil && *e.Description != "" {
		msg = fmt.Sprintf("%s: %s", e.Type, *e.Description)
	}
	if len(e.Properties) > 0 {
		msg = fmt.Sprintf("%s (properties: %v)", msg, e.Properties)
	}
	return msg
}
