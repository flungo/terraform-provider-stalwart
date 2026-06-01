// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package client

import "strings"

// idAlphabet is the custom base32 alphabet Stalwart uses to encode object ids.
// An Id is a u64 rendered with this alphabet (lowercase letters plus the digits
// 7,9,2,0,1,3), so a valid id is a non-empty string drawn entirely from it and
// at most 13 characters long (ceil(64/5)).
//
// VERIFIED against stalwartlabs/stalwart: BASE32_ALPHABET in
// crates/utils/src/codec/base32_custom.rs and the Id encoder in
// crates/types/src/id.rs. Note: Stalwart ids are NOT ULIDs.
const idAlphabet = "abcdefghijklmnopqrstuvwxyz792013"

// maxIDLength bounds a base32-encoded u64.
const maxIDLength = 13

// IsID reports whether s is syntactically a Stalwart object id. It is used to
// distinguish an opaque object id from a human-friendly reference (a domain
// name, email address, or role description) in import ids and domain
// references — those always contain a character outside the id alphabet (`.`,
// `@`, whitespace, or an uppercase letter), so they are never misclassified as
// ids.
func IsID(s string) bool {
	if s == "" || len(s) > maxIDLength {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune(idAlphabet, r) {
			return false
		}
	}
	return true
}
