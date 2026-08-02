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

func TestOrderedStringSetMarshal(t *testing.T) {
	cases := map[string]struct {
		in   OrderedStringSet
		want string
	}{
		"empty":  {OrderedStringSet{}, `{}`},
		"nil":    {nil, `{}`},
		"single": {OrderedStringSet{"a.com"}, `{"a.com":true}`},
		// Order is preserved verbatim, not sorted. This is the whole point of
		// the type: Stalwart makes the first entry of
		// certificateManagement.subjectAlternativeNames the certificate's CN.
		"preserves order": {
			OrderedStringSet{"mail", "autoconfig", "autodiscover"},
			`{"mail":true,"autoconfig":true,"autodiscover":true}`,
		},
		// Mirrors the server's push()-with-containment-check.
		"drops duplicates keeping first": {
			OrderedStringSet{"mail", "autoconfig", "mail"},
			`{"mail":true,"autoconfig":true}`,
		},
		"escapes keys": {OrderedStringSet{`a"b`}, `{"a\"b":true}`},
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

// TestOrderedStringSetVsStringSet pins the difference between the two types, so
// that a future refactor collapsing them fails loudly. StringSet marshals via a
// Go map, and encoding/json sorts map keys, which would silently reorder a SAN
// list and change the issued certificate's Subject Common Name.
func TestOrderedStringSetVsStringSet(t *testing.T) {
	in := []string{"mail", "autoconfig"}

	sorted, err := json.Marshal(StringSet(in))
	if err != nil {
		t.Fatalf("marshal StringSet: %v", err)
	}
	if string(sorted) != `{"autoconfig":true,"mail":true}` {
		t.Errorf("StringSet no longer sorts: got %s", sorted)
	}

	ordered, err := json.Marshal(OrderedStringSet(in))
	if err != nil {
		t.Fatalf("marshal OrderedStringSet: %v", err)
	}
	if string(ordered) != `{"mail":true,"autoconfig":true}` {
		t.Errorf("OrderedStringSet did not preserve order: got %s", ordered)
	}
}

func TestOrderedStringSetRoundTrip(t *testing.T) {
	// Object form decodes in document order, not sorted.
	var s OrderedStringSet
	if err := json.Unmarshal([]byte(`{"mail":true,"autoconfig":true}`), &s); err != nil {
		t.Fatalf("unmarshal object: %v", err)
	}
	if !reflect.DeepEqual([]string(s), []string{"mail", "autoconfig"}) {
		t.Errorf("object form decoded to %v, want [mail autoconfig]", s)
	}

	// Empty object decodes to an empty, non-nil slice.
	var empty OrderedStringSet
	if err := json.Unmarshal([]byte(`{}`), &empty); err != nil {
		t.Fatalf("unmarshal empty: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("empty object decoded to %v", empty)
	}

	// Array form is also accepted defensively.
	var arr OrderedStringSet
	if err := json.Unmarshal([]byte(`["x","y"]`), &arr); err != nil {
		t.Fatalf("unmarshal array: %v", err)
	}
	if !reflect.DeepEqual([]string(arr), []string{"x", "y"}) {
		t.Errorf("array form decoded to %v", arr)
	}

	// Marshal(Unmarshal(x)) == x for the wire form.
	const wire = `{"mail":true,"autoconfig":true,"autodiscover":true}`
	var rt OrderedStringSet
	if err := json.Unmarshal([]byte(wire), &rt); err != nil {
		t.Fatalf("unmarshal round trip: %v", err)
	}
	got, err := json.Marshal(rt)
	if err != nil {
		t.Fatalf("marshal round trip: %v", err)
	}
	if string(got) != wire {
		t.Errorf("round trip: got %s, want %s", got, wire)
	}
}

func TestOrderedStringSetUnmarshalRejectsNonObject(t *testing.T) {
	var s OrderedStringSet
	if err := json.Unmarshal([]byte(`"nope"`), &s); err == nil {
		t.Error("expected an error decoding a JSON string")
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
