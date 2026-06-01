#!/usr/bin/env bash
# Copyright (c) Fabrizio Lungo
# SPDX-License-Identifier: MPL-2.0
#
# Enforce a minimum total statement coverage from a Go coverage profile.
#
# Usage: coverage-check.sh <profile> <min-percent>
#   profile      path to a `go test -coverprofile` file
#   min-percent  minimum acceptable total coverage (integer or decimal)
#
# Prints the per-function summary and the total, then exits non-zero if the
# total is below the threshold.
set -euo pipefail

profile="${1:?usage: coverage-check.sh <profile> <min-percent>}"
threshold="${2:?usage: coverage-check.sh <profile> <min-percent>}"

if [[ ! -s "$profile" ]]; then
  echo "coverage-check: profile '$profile' is missing or empty" >&2
  exit 2
fi

# `go tool cover -func` ends with a line like:  total:   (statements)   87.3%
total="$(go tool cover -func="$profile" | awk '/^total:/ {gsub(/%/,"",$NF); print $NF}')"
if [[ -z "$total" ]]; then
  echo "coverage-check: could not determine total coverage from '$profile'" >&2
  exit 2
fi

echo "Total coverage: ${total}% (threshold: ${threshold}%)"

# Compare with awk to avoid relying on bc / floating-point shell arithmetic.
if awk -v t="$total" -v min="$threshold" 'BEGIN { exit !(t + 0 < min + 0) }'; then
  echo "FAIL: coverage ${total}% is below the required ${threshold}%" >&2
  exit 1
fi
echo "OK: coverage ${total}% meets the required ${threshold}%"
