// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

// Stalwart's management JMAP objects do not represent collection-valued
// properties as JSON arrays. They use two distinct JSON object ("map")
// encodings, mirroring the server's Rust `Map<T>` and `List<T>` types:
//
//   - Map<T>  -> {"<value>": true, ...}     (a set; keys are the values)
//   - List<T> -> {"0": <item>, "1": <item>} (an ordered list keyed by index)
//
// Sending a JSON array for either is rejected by the server with
// `invalidPatch: Invalid value for object property`. The types below implement
// the correct encodings.

// StringSet models a Stalwart `Map<T>` of scalar values (e.g. domain aliases,
// member group ids, recipients, role ids, permissions). On the wire it is a
// JSON object mapping each value to `true`.
type StringSet []string

// MarshalJSON encodes the set as {"value": true, ...}. A nil slice still encodes
// as an empty object {}, which is what the server expects for "no items".
func (s StringSet) MarshalJSON() ([]byte, error) {
	m := make(map[string]bool, len(s))
	for _, v := range s {
		m[v] = true
	}
	return json.Marshal(m)
}

// UnmarshalJSON accepts the object form {"value": true, ...} and, defensively,
// a JSON array of strings. Keys/elements are returned sorted for stable state.
func (s *StringSet) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '[' {
		var arr []string
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		*s = arr
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("decoding string set: %w", err)
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	*s = out
	return nil
}

// IndexList models a Stalwart `List<T>` of object items (e.g. account
// credentials, email aliases). On the wire it is a JSON object keyed by the
// stringified item index: {"0": <item>, "1": <item>, ...}.
type IndexList[T any] []T

// MarshalJSON encodes the list as {"0": item, "1": item, ...}.
func (l IndexList[T]) MarshalJSON() ([]byte, error) {
	m := make(map[string]T, len(l))
	for i, item := range l {
		m[strconv.Itoa(i)] = item
	}
	return json.Marshal(m)
}

// UnmarshalJSON accepts the index-keyed object form and, defensively, a JSON
// array. Items are ordered by ascending integer key.
func (l *IndexList[T]) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '[' {
		var arr []T
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		*l = arr
		return nil
	}
	var m map[string]T
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("decoding index list: %w", err)
	}
	keys := make([]int, 0, len(m))
	idx := make(map[int]string, len(m))
	for k := range m {
		n, err := strconv.Atoi(k)
		if err != nil {
			return fmt.Errorf("decoding index list: non-integer key %q", k)
		}
		keys = append(keys, n)
		idx[n] = k
	}
	sort.Ints(keys)
	out := make([]T, 0, len(keys))
	for _, n := range keys {
		out = append(out, m[idx[n]])
	}
	*l = out
	return nil
}
