// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// useStateForUnknownUnlessChanged is a Set plan modifier that copies the prior
// state value into an unknown planned value (like the stock
// setplanmodifier.UseStateForUnknown) — but only while a "trigger" attribute is
// unchanged. When the trigger differs between state and plan, the value is left
// unknown so the server recomputes it.
//
// This is needed for collection attributes whose server-side value is coupled to
// a discriminator on the same object. For example, an account's `role_ids` lives
// inside the `Custom` variant of `role`: changing `role` from `Custom` to
// `User`/`Default` clears `role_ids` server-side, so blindly reusing the prior
// state (plain UseStateForUnknown) would plan a stale value that the apply then
// contradicts ("inconsistent result after apply").
type useStateForUnknownUnlessChanged struct {
	trigger path.Path
}

func useStateForUnknownUnlessTrigger(trigger path.Path) planmodifier.Set {
	return useStateForUnknownUnlessChanged{trigger: trigger}
}

func (m useStateForUnknownUnlessChanged) Description(_ context.Context) string {
	return "Use prior state for an unknown value, unless the trigger attribute changed."
}

func (m useStateForUnknownUnlessChanged) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m useStateForUnknownUnlessChanged) PlanModifySet(ctx context.Context, req planmodifier.SetRequest, resp *planmodifier.SetResponse) {
	// Only act on unknown planned values; a known config value is authoritative.
	if !resp.PlanValue.IsUnknown() {
		return
	}
	// On create (no prior state) there is nothing to copy.
	if req.StateValue.IsNull() {
		return
	}

	var stateTrigger, planTrigger types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, m.trigger, &stateTrigger)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, m.trigger, &planTrigger)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If the trigger is changing, leave the value unknown so the server decides.
	if !planTrigger.Equal(stateTrigger) {
		return
	}
	resp.PlanValue = req.StateValue
}
