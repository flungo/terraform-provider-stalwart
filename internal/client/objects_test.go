// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestDomainUnmarshalAutomaticDNS is a regression test for a domain whose
// dnsManagement is the "Automatic" variant with a populated publishRecords set.
//
// Stalwart models publishRecords as Map<DnsRecordType>, which serializes on the
// wire as a JSON object {"dkim": true, "spf": true, ...} (the same Map<T>
// encoding as StringSet). It was previously modeled as a Go bool, so reading any
// auto-DNS domain failed with:
//
//	json: cannot unmarshal object into Go struct field
//	TypedRef.dnsManagement.publishRecords of type bool
//
// blocking import/plan/apply for those domains.
func TestDomainUnmarshalAutomaticDNS(t *testing.T) {
	// A Domain/get response for a domain with Automatic DNS, mirroring the exact
	// shape returned by a production server: every record type publishing, and
	// origin sent as JSON null (the server emits it even in the Automatic case).
	const wire = `{
		"id": "a01",
		"name": "example.test",
		"certificateManagement": {
			"acmeProviderId": "acme01",
			"subjectAlternativeNames": {"mail": true, "autoconfig": true},
			"@type": "Automatic"
		},
		"dnsManagement": {
			"dnsServerId": "dns01",
			"origin": null,
			"publishRecords": {
				"autoConfig": true,
				"autoConfigLegacy": true,
				"autoDiscover": true,
				"caa": true,
				"dkim": true,
				"dmarc": true,
				"mtaSts": true,
				"mx": true,
				"spf": true,
				"srv": true,
				"tlsRpt": true
			},
			"@type": "Automatic"
		}
	}`

	var d Domain
	if err := json.Unmarshal([]byte(wire), &d); err != nil {
		t.Fatalf("unmarshal Domain with Automatic DNS: %v", err)
	}

	if d.DNSManagement == nil {
		t.Fatal("dnsManagement: got nil")
	}
	if d.DNSManagement.Type != "Automatic" {
		t.Errorf("dnsManagement.@type: got %q, want Automatic", d.DNSManagement.Type)
	}
	if d.DNSManagement.DNSServerID == nil || *d.DNSManagement.DNSServerID != "dns01" {
		t.Errorf("dnsManagement.dnsServerId: got %v, want dns01", d.DNSManagement.DNSServerID)
	}
	if d.DNSManagement.Origin != nil {
		t.Errorf("dnsManagement.origin: got %q, want nil (null on the wire)", *d.DNSManagement.Origin)
	}
	if d.DNSManagement.PublishRecords == nil {
		t.Fatal("dnsManagement.publishRecords: got nil")
	}
	// StringSet decodes the Map<T> object form to a sorted slice.
	got := []string(*d.DNSManagement.PublishRecords)
	want := []string{
		"autoConfig", "autoConfigLegacy", "autoDiscover", "caa", "dkim",
		"dmarc", "mtaSts", "mx", "spf", "srv", "tlsRpt",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dnsManagement.publishRecords: got %v, want %v", got, want)
	}
}

// TestDomainMarshalAutomaticDNS verifies the create/update payload encodes
// publishRecords back as the Map<T> object form the server expects.
func TestDomainMarshalAutomaticDNS(t *testing.T) {
	records := StringSet{"dkim", "mx"}
	d := Domain{
		DNSManagement: &TypedRef{
			Type:           "Automatic",
			DNSServerID:    strp("d99"),
			Origin:         strp("flungo.net"),
			PublishRecords: &records,
		},
	}
	got, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Round-trip back into a TypedRef to assert the publishRecords encoding,
	// independent of map key ordering in the raw JSON.
	var rt Domain
	if err := json.Unmarshal(got, &rt); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if rt.DNSManagement.PublishRecords == nil {
		t.Fatal("round-trip publishRecords: got nil")
	}
	if rec := []string(*rt.DNSManagement.PublishRecords); !reflect.DeepEqual(rec, []string{"dkim", "mx"}) {
		t.Errorf("round-trip publishRecords: got %v, want [dkim mx]", rec)
	}
}

// TestDomainUnmarshalManualDNS confirms the Manual variant (no publishRecords)
// still decodes cleanly.
func TestDomainUnmarshalManualDNS(t *testing.T) {
	const wire = `{"id":"a01","name":"flungo.net","dnsManagement":{"@type":"Manual"}}`
	var d Domain
	if err := json.Unmarshal([]byte(wire), &d); err != nil {
		t.Fatalf("unmarshal Domain with Manual DNS: %v", err)
	}
	if d.DNSManagement == nil || d.DNSManagement.Type != "Manual" {
		t.Fatalf("dnsManagement: got %+v, want @type Manual", d.DNSManagement)
	}
	if d.DNSManagement.PublishRecords != nil {
		t.Errorf("dnsManagement.publishRecords: got %v, want nil", *d.DNSManagement.PublishRecords)
	}
}

func strp(s string) *string { return &s }
