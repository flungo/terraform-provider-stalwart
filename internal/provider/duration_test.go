// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import "testing"

func TestParseDuration(t *testing.T) {
	ok := map[string]int64{
		"90d":   90 * 24 * 60 * 60 * 1000,
		"1h":    60 * 60 * 1000,
		"30m":   30 * 60 * 1000,
		"45s":   45 * 1000,
		"500ms": 500,
		"250":   250, // no unit means milliseconds
	}
	for in, want := range ok {
		got, err := parseDuration(in)
		if err != nil {
			t.Errorf("parseDuration(%q) unexpected error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseDuration(%q) = %d, want %d", in, got, want)
		}
	}

	for _, bad := range []string{"90x", "abc", "10y"} {
		if _, err := parseDuration(bad); err == nil {
			t.Errorf("parseDuration(%q) expected error, got nil", bad)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[int64]string{
		90 * 24 * 60 * 60 * 1000: "90d",
		60 * 60 * 1000:           "1h",
		90 * 1000:                "90s", // 90s is not a whole minute
		1500:                     "1500ms",
	}
	for in, want := range cases {
		if got := formatDuration(in); got != want {
			t.Errorf("formatDuration(%d) = %q, want %q", in, got, want)
		}
	}

	// Round-trip the canonical example value.
	ms, err := parseDuration("90d")
	if err != nil {
		t.Fatalf("parseDuration: %v", err)
	}
	if got := formatDuration(ms); got != "90d" {
		t.Errorf("round-trip 90d = %q", got)
	}
}
