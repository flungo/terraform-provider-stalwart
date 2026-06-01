// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestStringSetMarshal(t *testing.T) {
	cases := map[string]struct {
		in   StringSet
		want string
	}{
		"empty":  {StringSet{}, `{}`},
		"nil":    {nil, `{}`},
		"single": {StringSet{"a.com"}, `{"a.com":true}`},
	}
	for name, tc := range cases {
		got, err := json.Marshal(tc.in)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s: got %s, want %s", name, got, tc.want)
		}
	}
}

func TestStringSetRoundTrip(t *testing.T) {
	// Object form (the wire format) decodes to a sorted slice.
	var s StringSet
	if err := json.Unmarshal([]byte(`{"b.com":true,"a.com":true}`), &s); err != nil {
		t.Fatalf("unmarshal object: %v", err)
	}
	if !reflect.DeepEqual([]string(s), []string{"a.com", "b.com"}) {
		t.Errorf("object form decoded to %v", s)
	}

	// Array form is also accepted defensively.
	var s2 StringSet
	if err := json.Unmarshal([]byte(`["x","y"]`), &s2); err != nil {
		t.Fatalf("unmarshal array: %v", err)
	}
	if !reflect.DeepEqual([]string(s2), []string{"x", "y"}) {
		t.Errorf("array form decoded to %v", s2)
	}
}

func TestIndexListMarshal(t *testing.T) {
	type item struct {
		Type string `json:"@type"`
	}
	l := IndexList[item]{{Type: "Password"}, {Type: "ApiKey"}}
	got, err := json.Marshal(l)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"0":{"@type":"Password"},"1":{"@type":"ApiKey"}}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}

	// Empty encodes as {}.
	got, _ = json.Marshal(IndexList[item]{})
	if string(got) != `{}` {
		t.Errorf("empty got %s, want {}", got)
	}
}

func TestIndexListUnmarshalOrdered(t *testing.T) {
	type item struct {
		Type string `json:"@type"`
	}
	var l IndexList[item]
	// Keys deliberately out of order; result must be index-ordered.
	if err := json.Unmarshal([]byte(`{"1":{"@type":"b"},"0":{"@type":"a"}}`), &l); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(l) != 2 || l[0].Type != "a" || l[1].Type != "b" {
		t.Errorf("decoded out of order: %+v", l)
	}
}
