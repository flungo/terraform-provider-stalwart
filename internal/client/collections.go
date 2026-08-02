// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
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

// OrderedStringSet models a Stalwart `Map<T>` whose element order is
// significant. It shares StringSet's wire format ({"<value>": true, ...}) but
// preserves the caller's order in both directions.
//
// Stalwart's `Map<T>` is a `Vec<T>` behind a JSON-object encoding (upstream
// crates/registry/src/types/map.rs): push() appends after a containment check
// and nothing sorts, so element order round-trips through the server intact.
// That matters wherever the server treats the first element specially — most
// notably `certificateManagement.subjectAlternativeNames`, whose first entry
// becomes the issued certificate's Subject Common Name.
//
// StringSet cannot serve those fields: it marshals via map[string]bool, and
// encoding/json sorts map keys, so the order reaching the server is always
// alphabetical no matter what the practitioner configured.
type OrderedStringSet []string

// MarshalJSON encodes the set as {"value": true, ...}, preserving slice order.
// Duplicates are dropped, keeping the first occurrence, mirroring the server's
// own push()-with-containment-check. A nil slice encodes as {}.
func (s OrderedStringSet) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	seen := make(map[string]struct{}, len(s))
	for _, v := range s {
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		if len(seen) > 1 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("encoding ordered string set: %w", err)
		}
		buf.Write(key)
		buf.WriteString(":true")
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// UnmarshalJSON accepts the object form {"value": true, ...} and, defensively,
// a JSON array of strings. Object keys are returned in document order rather
// than sorted, which is what makes the round-trip order-preserving.
func (s *OrderedStringSet) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '[' {
		var arr []string
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		*s = arr
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("decoding ordered string set: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("decoding ordered string set: expected object, got %v", tok)
	}
	out := make([]string, 0, 4)
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("decoding ordered string set: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return fmt.Errorf("decoding ordered string set: non-string key %v", keyTok)
		}
		// The value is always true on the wire; decode and discard it so the
		// decoder advances to the next key.
		var discard json.RawMessage
		if err := dec.Decode(&discard); err != nil {
			return fmt.Errorf("decoding ordered string set: %w", err)
		}
		out = append(out, key)
	}
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
