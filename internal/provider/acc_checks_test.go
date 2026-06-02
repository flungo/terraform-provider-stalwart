// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/flungo/terraform-provider-stalwart/internal/client"
)

// The helpers here read an object back through a direct JMAP client (not the
// provider) and assert its fields, so write-path correctness is verified
// independently of the provider's read path. Each returns a
// resource.TestCheckFunc that resolves the resource's server id from Terraform
// state, fetches the live object, and runs the supplied assertions against it.

// resourceID extracts the `id` attribute of the named resource from state.
func resourceID(s *terraform.State, resourceName string) (string, error) {
	rs, ok := s.RootModule().Resources[resourceName]
	if !ok {
		return "", fmt.Errorf("resource %s not found in state", resourceName)
	}
	id := rs.Primary.Attributes["id"]
	if id == "" {
		return "", fmt.Errorf("resource %s has no id in state", resourceName)
	}
	return id, nil
}

// checkServerDomain fetches the domain by its state id via a direct client and
// runs assert against the server's view of the object.
func checkServerDomain(c *client.Client, resourceName string, assert func(client.Domain) error) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := resourceID(s, resourceName)
		if err != nil {
			return err
		}
		var dom client.Domain
		if err := c.GetOne(accCtx(), client.TypeDomain, id, &dom); err != nil {
			return fmt.Errorf("reading domain %s from server: %w", id, err)
		}
		return assert(dom)
	}
}

// checkServerAccount fetches the account/group by its state id via a direct
// client and runs assert against the server's view.
func checkServerAccount(c *client.Client, resourceName string, assert func(client.Account) error) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := resourceID(s, resourceName)
		if err != nil {
			return err
		}
		var acct client.Account
		if err := c.GetOne(accCtx(), client.TypeAccount, id, &acct); err != nil {
			return fmt.Errorf("reading account %s from server: %w", id, err)
		}
		return assert(acct)
	}
}

// checkServerMailingList fetches the mailing list by its state id.
func checkServerMailingList(c *client.Client, resourceName string, assert func(client.MailingList) error) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := resourceID(s, resourceName)
		if err != nil {
			return err
		}
		var list client.MailingList
		if err := c.GetOne(accCtx(), client.TypeMailingList, id, &list); err != nil {
			return fmt.Errorf("reading mailing list %s from server: %w", id, err)
		}
		return assert(list)
	}
}

// checkServerRole fetches the role by its state id.
func checkServerRole(c *client.Client, resourceName string, assert func(client.Role) error) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := resourceID(s, resourceName)
		if err != nil {
			return err
		}
		var role client.Role
		if err := c.GetOne(accCtx(), client.TypeRole, id, &role); err != nil {
			return fmt.Errorf("reading role %s from server: %w", id, err)
		}
		return assert(role)
	}
}

// checkServerDkim fetches the DKIM signature by its state id.
func checkServerDkim(c *client.Client, resourceName string, assert func(client.DkimSignature) error) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := resourceID(s, resourceName)
		if err != nil {
			return err
		}
		var sig client.DkimSignature
		if err := c.GetOne(accCtx(), client.TypeDkimSignature, id, &sig); err != nil {
			return fmt.Errorf("reading DKIM signature %s from server: %w", id, err)
		}
		return assert(sig)
	}
}

// checkServerDnsServer fetches the DNS server by its state id.
func checkServerDnsServer(c *client.Client, resourceName string, assert func(client.DnsServer) error) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := resourceID(s, resourceName)
		if err != nil {
			return err
		}
		var srv client.DnsServer
		if err := c.GetOne(accCtx(), client.TypeDnsServer, id, &srv); err != nil {
			return fmt.Errorf("reading DNS server %s from server: %w", id, err)
		}
		return assert(srv)
	}
}

// checkServerAcmeProvider fetches the ACME provider by its state id.
func checkServerAcmeProvider(c *client.Client, resourceName string, assert func(client.AcmeProvider) error) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := resourceID(s, resourceName)
		if err != nil {
			return err
		}
		var p client.AcmeProvider
		if err := c.GetOne(accCtx(), client.TypeAcmeProvider, id, &p); err != nil {
			return fmt.Errorf("reading ACME provider %s from server: %w", id, err)
		}
		return assert(p)
	}
}

// checkServerDirectory fetches the directory by its state id.
func checkServerDirectory(c *client.Client, resourceName string, assert func(client.Directory) error) resource.TestCheckFunc { //nolint:unparam
	return func(s *terraform.State) error {
		id, err := resourceID(s, resourceName)
		if err != nil {
			return err
		}
		var d client.Directory
		if err := c.GetOne(accCtx(), client.TypeDirectory, id, &d); err != nil {
			return fmt.Errorf("reading directory %s from server: %w", id, err)
		}
		return assert(d)
	}
}

// checkServerNetworkListener fetches the network listener by its state id.
func checkServerNetworkListener(c *client.Client, resourceName string, assert func(client.NetworkListener) error) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := resourceID(s, resourceName)
		if err != nil {
			return err
		}
		var l client.NetworkListener
		if err := c.GetOne(accCtx(), client.TypeNetworkListener, id, &l); err != nil {
			return fmt.Errorf("reading network listener %s from server: %w", id, err)
		}
		return assert(l)
	}
}

// --- small assertion helpers used inside the assert callbacks ----------------

// firstErr returns the first non-nil error from errs, or nil. It lets an assert
// callback report the earliest field mismatch concisely.
func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// wantVariant asserts that a *TypedRef's @type discriminator equals want.
func wantVariant(field string, got *client.TypedRef, want string) error {
	if got == nil {
		return fmt.Errorf("%s: got nil, want @type %q", field, want)
	}
	if got.Type != want {
		return fmt.Errorf("%s: got @type %q, want %q", field, got.Type, want)
	}
	return nil
}

// wantStr asserts that a *string field equals want.
func wantStr(field string, got *string, want string) error {
	if got == nil {
		return fmt.Errorf("%s: got nil, want %q", field, want)
	}
	if *got != want {
		return fmt.Errorf("%s: got %q, want %q", field, *got, want)
	}
	return nil
}

// wantStrNil asserts that a *string field is nil/unset.
func wantStrNil(field string, got *string) error {
	if got != nil {
		return fmt.Errorf("%s: got %q, want unset", field, *got)
	}
	return nil
}

// wantBool asserts that a *bool field equals want.
func wantBool(field string, got *bool, want bool) error {
	if got == nil {
		return fmt.Errorf("%s: got nil, want %v", field, want)
	}
	if *got != want {
		return fmt.Errorf("%s: got %v, want %v", field, *got, want)
	}
	return nil
}

// wantExpire asserts that a *int64 millisecond field equals want.
func wantExpire(field string, got *int64, want int64) error {
	if got == nil {
		return fmt.Errorf("%s: got nil, want %d", field, want)
	}
	if *got != want {
		return fmt.Errorf("%s: got %d, want %d", field, *got, want)
	}
	return nil
}

// wantQuota asserts that the quotas map has key with value want.
func wantQuota(key string, quotas map[string]int64, want int64) error {
	got, ok := quotas[key]
	if !ok {
		return fmt.Errorf("quotas[%s]: missing, want %d", key, want)
	}
	if got != want {
		return fmt.Errorf("quotas[%s]: got %d, want %d", key, got, want)
	}
	return nil
}

// wantRoleType asserts the @type discriminator of a *Roles union.
func wantRoleType(got *client.Roles, want string) error {
	if got == nil {
		return fmt.Errorf("roles: got nil, want @type %q", want)
	}
	if got.Type != want {
		return fmt.Errorf("roles.@type: got %q, want %q", got.Type, want)
	}
	return nil
}

// checkAccountLinksResolved independently verifies that the account's server-side
// id references (domainId, memberGroupIds, roles.roleIds) point at the ids of the
// referenced resources as recorded in Terraform state — i.e. the provider sent
// the right opaque ids, not just well-formed payloads.
func checkAccountLinksResolved(c *client.Client, accountRes, groupRes, roleRes string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		acctID, err := resourceID(s, accountRes)
		if err != nil {
			return err
		}
		groupID, err := resourceID(s, groupRes)
		if err != nil {
			return err
		}
		roleID, err := resourceID(s, roleRes)
		if err != nil {
			return err
		}
		domainID, err := resourceID(s, "stalwart_domain.test")
		if err != nil {
			return err
		}

		var acct client.Account
		if err := c.GetOne(accCtx(), client.TypeAccount, acctID, &acct); err != nil {
			return fmt.Errorf("reading account %s: %w", acctID, err)
		}
		if err := wantStr("domainId", acct.DomainID, domainID); err != nil {
			return err
		}
		if err := wantSet("memberGroupIds", acct.MemberGroupIDs, groupID); err != nil {
			return err
		}
		if acct.Roles == nil {
			return fmt.Errorf("roles: got nil, want roleIds [%s]", roleID)
		}
		return wantSetVal("roles.roleIds", acct.Roles.RoleIDs, roleID)
	}
}

// wantSetVal is wantSet for a non-pointer StringSet.
func wantSetVal(field string, got client.StringSet, want ...string) error {
	return wantSet(field, &got, want...)
}

// wantSetPtr asserts that a *StringSet field contains exactly want. (Identical
// to wantSet; named for readability at call sites operating on object fields.)
func wantSetPtr(field string, got *client.StringSet, want ...string) error {
	return wantSet(field, got, want...)
}

// wantSet asserts that a *StringSet contains exactly want (order-insensitive).
func wantSet(field string, got *client.StringSet, want ...string) error {
	var have []string
	if got != nil {
		have = []string(*got)
	}
	gotSorted := append([]string(nil), have...)
	wantSorted := append([]string(nil), want...)
	sort.Strings(gotSorted)
	sort.Strings(wantSorted)
	if fmt.Sprint(gotSorted) != fmt.Sprint(wantSorted) {
		return fmt.Errorf("%s: got %v, want %v", field, gotSorted, wantSorted)
	}
	return nil
}
