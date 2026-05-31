// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"fmt"
)

// callID is the constant call identifier used for single-method requests.
const callID = "c0"

// ErrNotFound is returned by GetOne when the requested object does not exist.
var ErrNotFound = fmt.Errorf("object not found")

// method returns the wire method name for an object type and operation, e.g.
// method("Domain", "get") == "x:Domain/get".
func method(objType, op string) string {
	return MethodPrefix + objType + "/" + op
}

// Get retrieves objects of the given type by their ids.
func (c *Client) Get(ctx context.Context, objType string, ids []string) (*GetResponse, error) {
	args := map[string]any{"ids": ids}
	responses, err := c.call(ctx, Invocation{Name: method(objType, "get"), Args: args, CallID: callID})
	if err != nil {
		return nil, err
	}
	var result GetResponse
	if err := decodeResult(responses, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetOne retrieves a single object by id and decodes it into dst. It returns
// ErrNotFound if the object does not exist.
func (c *Client) GetOne(ctx context.Context, objType, id string, dst any) error {
	resp, err := c.Get(ctx, objType, []string{id})
	if err != nil {
		return err
	}
	if len(resp.List) == 0 {
		return ErrNotFound
	}
	return json.Unmarshal(resp.List[0], dst)
}

// Create creates a single object and returns its server-assigned id. The
// created object's server-set properties are decoded into created if non-nil.
func (c *Client) Create(ctx context.Context, objType string, obj any, created any) (string, error) {
	const creationID = "new"
	args := map[string]any{
		"create": map[string]any{creationID: obj},
	}
	responses, err := c.call(ctx, Invocation{Name: method(objType, "set"), Args: args, CallID: callID})
	if err != nil {
		return "", err
	}
	var result SetResponse
	if err := decodeResult(responses, &result); err != nil {
		return "", err
	}
	if setErr, ok := result.NotCreated[creationID]; ok {
		return "", fmt.Errorf("creating %s: %w", objType, setErr)
	}
	raw, ok := result.Created[creationID]
	if !ok {
		return "", fmt.Errorf("creating %s: server did not return the created object", objType)
	}

	// The created response contains (at least) the server-assigned id along
	// with any other server-set properties.
	var idHolder struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &idHolder); err != nil {
		return "", fmt.Errorf("decoding created %s id: %w", objType, err)
	}
	if idHolder.ID == "" {
		return "", fmt.Errorf("creating %s: server returned an empty id", objType)
	}
	if created != nil {
		if err := json.Unmarshal(raw, created); err != nil {
			return "", fmt.Errorf("decoding created %s: %w", objType, err)
		}
	}
	return idHolder.ID, nil
}

// Update applies a partial update (patch) to the object with the given id.
func (c *Client) Update(ctx context.Context, objType, id string, patch any) error {
	args := map[string]any{
		"update": map[string]any{id: patch},
	}
	responses, err := c.call(ctx, Invocation{Name: method(objType, "set"), Args: args, CallID: callID})
	if err != nil {
		return err
	}
	var result SetResponse
	if err := decodeResult(responses, &result); err != nil {
		return err
	}
	if setErr, ok := result.NotUpdated[id]; ok {
		return fmt.Errorf("updating %s %s: %w", objType, id, setErr)
	}
	return nil
}

// Destroy deletes the object with the given id.
func (c *Client) Destroy(ctx context.Context, objType, id string) error {
	args := map[string]any{
		"destroy": []string{id},
	}
	responses, err := c.call(ctx, Invocation{Name: method(objType, "set"), Args: args, CallID: callID})
	if err != nil {
		return err
	}
	var result SetResponse
	if err := decodeResult(responses, &result); err != nil {
		return err
	}
	if setErr, ok := result.NotDestroyed[id]; ok {
		return fmt.Errorf("destroying %s %s: %w", objType, id, setErr)
	}
	return nil
}

// Query returns the ids of objects of the given type matching the filter. A nil
// filter matches all objects.
func (c *Client) Query(ctx context.Context, objType string, filter any) ([]string, error) {
	args := map[string]any{}
	if filter != nil {
		args["filter"] = filter
	}
	responses, err := c.call(ctx, Invocation{Name: method(objType, "query"), Args: args, CallID: callID})
	if err != nil {
		return nil, err
	}
	var result QueryResponse
	if err := decodeResult(responses, &result); err != nil {
		return nil, err
	}
	return result.IDs, nil
}

// QueryOne returns the single id matching the filter, or ErrNotFound if there
// are no matches. It returns an error if the filter matches more than one
// object.
func (c *Client) QueryOne(ctx context.Context, objType string, filter any) (string, error) {
	ids, err := c.Query(ctx, objType, filter)
	if err != nil {
		return "", err
	}
	switch len(ids) {
	case 0:
		return "", ErrNotFound
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("expected exactly one %s to match, found %d", objType, len(ids))
	}
}

// decodeResult extracts the arguments of the single expected method response
// and decodes them into dst.
func decodeResult(responses []rawInvocation, dst any) error {
	if len(responses) == 0 {
		return fmt.Errorf("jmap response contained no method responses")
	}
	return json.Unmarshal(responses[0].Args, dst)
}
