// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package acctest

import (
	"strings"
	"testing"
)

func TestImageOverride(t *testing.T) {
	t.Setenv(imageEnv, "")
	if got := image(); got != DefaultImage {
		t.Errorf("image() = %q, want default %q", got, DefaultImage)
	}

	t.Setenv(imageEnv, "stalwartlabs/stalwart:v0.17")
	if got := image(); got != "stalwartlabs/stalwart:v0.17" {
		t.Errorf("image() = %q, want override", got)
	}
}

// TestBootstrapScript guards the shape of the in-container startup script: it
// must write the minimal RocksDB DataStore config and exec the server pointed at
// it, so that the server comes up in recovery mode rather than the interactive
// bootstrap wizard.
func TestBootstrapScript(t *testing.T) {
	script := bootstrapScript()
	for _, want := range []string{
		`"@type":"RocksDb"`,
		`/tmp/stalwart-config.json`,
		`command -v stalwart`,
		`exec "$bin" --config`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("bootstrap script missing %q\nscript: %s", want, script)
		}
	}
}
