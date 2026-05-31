// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package client

import "strings"

// ulidLength is the length of a canonical ULID string representation.
const ulidLength = 26

// crockfordBase32 is the alphabet used by ULID's Crockford base32 encoding,
// excluding the letters I, L, O, and U to avoid ambiguity.
const crockfordBase32 = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// IsULID reports whether s is a syntactically valid ULID. Stalwart identifies
// management objects by ULID, so this is used to distinguish an opaque object
// id from a human-friendly name (such as a domain name or email address) in
// import ids and domain references.
func IsULID(s string) bool {
	if len(s) != ulidLength {
		return false
	}
	upper := strings.ToUpper(s)
	for _, r := range upper {
		if !strings.ContainsRune(crockfordBase32, r) {
			return false
		}
	}
	return true
}
