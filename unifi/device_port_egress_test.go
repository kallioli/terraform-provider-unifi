package unifi

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

func egressObject(t *testing.T, enabled bool, kbps types.Int64) types.Object {
	t.Helper()
	obj, d := types.ObjectValue(portEgressRateLimitAttrTypes(), map[string]attr.Value{
		"enabled": types.BoolValue(enabled),
		"kbps":    kbps,
	})
	if d.HasError() {
		t.Fatalf("egress object: %v", d)
	}
	return obj
}

// An update of a port whose limit is disabled plans kbps unknown (it is
// Optional+Computed): sending it as 0 makes the controller reject the PUT
// with api.err.InvalidPayload (#763). A 0 read back from an empty stored value
// must not be sent either.
func TestPortOverrideModelToAPI_egressKbps(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		kbps    types.Int64
		want    *int64
	}{
		{"unknown", false, types.Int64Unknown(), nil},
		{"null", false, types.Int64Null(), nil},
		{"zero", false, types.Int64Value(0), nil},
		{"set", true, types.Int64Value(1000), new(int64(1000))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := portOverrideModel{
				Index:           types.Int64Value(7),
				EgressRateLimit: egressObject(t, tc.enabled, tc.kbps),
			}
			po, d := portOverrideModelToAPI(context.Background(), model)
			if d.HasError() {
				t.Fatalf("portOverrideModelToAPI: %v", d)
			}
			switch {
			case tc.want == nil && po.EgressRateLimitKbps != nil:
				t.Errorf("EgressRateLimitKbps = %d, want unset", *po.EgressRateLimitKbps)
			case tc.want != nil && (po.EgressRateLimitKbps == nil ||
				*po.EgressRateLimitKbps != *tc.want):
				t.Errorf("EgressRateLimitKbps = %v, want %d", po.EgressRateLimitKbps, *tc.want)
			}
			gotEnabled := po.EgressRateLimitKbpsEnabled
			if gotEnabled == nil || *gotEnabled != tc.enabled {
				t.Errorf("EgressRateLimitKbpsEnabled = %v, want %v",
					po.EgressRateLimitKbpsEnabled, tc.enabled)
			}
		})
	}
}

// A port whose config leaves kbps unset keeps the rate the controller stores.
func Test_carryUnwritableFields_keepsStoredEgressKbps(t *testing.T) {
	stored := int64(2000)
	zero := int64(0)
	declared := int64(500)
	cases := []struct {
		name     string
		current  *int64
		declared *int64
		want     *int64
	}{
		{"unset keeps stored", &stored, nil, &stored},
		{"set wins", &stored, &declared, &declared},
		{"stored empty stays unset", &zero, nil, nil},
		{"nothing stored", nil, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := carryUnwritableFields(
				unifi.DevicePortOverrides{EgressRateLimitKbps: tc.current},
				unifi.DevicePortOverrides{EgressRateLimitKbps: tc.declared},
			)
			switch {
			case tc.want == nil && got.EgressRateLimitKbps != nil:
				t.Errorf("EgressRateLimitKbps = %d, want unset", *got.EgressRateLimitKbps)
			case tc.want != nil && (got.EgressRateLimitKbps == nil ||
				*got.EgressRateLimitKbps != *tc.want):
				t.Errorf("EgressRateLimitKbps = %v, want %d", got.EgressRateLimitKbps, *tc.want)
			}
		})
	}
}

// The scenario of #763: a port imported with its egress limit disabled, then
// updated on another attribute. The plan holds kbps unknown, and the PUT must
// not carry egress_rate_limit_kbps (the controller rejects 0).
func TestDevicePortUpdate_disabledEgressLimitSendsNoKbps(t *testing.T) {
	sw, client := newFakeSwitch(t, map[string]any{
		"port_idx": 2, "poe_mode": "off", "egress_rate_limit_kbps_enabled": false,
	})
	r := &devicePortResource{client: client}
	s := resourceSchema(t, r)
	prior := map[string]any{
		"id":                fakeSwitchMAC + "/2",
		"site":              "default",
		"device_mac":        fakeSwitchMAC,
		"index":             int64(2),
		"poe_mode":          "off",
		"egress_rate_limit": map[string]any{"enabled": false, "kbps": nil},
	}
	planned := map[string]any{}
	for k, v := range prior {
		planned[k] = v
	}
	planned["poe_mode"] = "auto"
	planned["egress_rate_limit"] = map[string]any{"enabled": false, "kbps": tfUnknown}
	config := map[string]any{}
	for k, v := range planned {
		config[k] = v
	}
	config["egress_rate_limit"] = map[string]any{"enabled": false, "kbps": nil}

	resp := &fwresource.UpdateResponse{State: tfState(t, s, prior), Identity: tfIdentity(t, r)}
	r.Update(context.Background(), fwresource.UpdateRequest{
		Plan:   tfPlan(t, s, planned),
		Config: tfConfig(t, s, config),
		State:  tfState(t, s, prior),
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()
	if len(sw.puts) == 0 {
		t.Fatal("no PUT sent")
	}
	po, _ := sw.puts[len(sw.puts)-1]["port_overrides"].([]any)
	entry := overrideAt(po, 2)
	if entry == nil {
		t.Fatalf("PUT has no entry for port 2: %v", po)
	}
	if v, ok := entry["egress_rate_limit_kbps"]; ok {
		t.Errorf("PUT sent egress_rate_limit_kbps = %v, want it absent", v)
	}
	if entry["poe_mode"] != "auto" {
		t.Errorf("poe_mode = %v, want auto", entry["poe_mode"])
	}
}
