package unifi

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// A network stored without ipv6_setting_preference reads it as null. An update
// that configures the ipv6 block without setting_preference must leave the
// null prior unknown: the controller can store a value of its own when it
// saves the network, and a planned known null would then fail the apply with
// "inconsistent result after apply".
func TestNetworkUpdatePlansUnknownForNullIPv6SettingPreference(t *testing.T) {
	doc := strings.Replace(networkTestDocument, `"ipv6_setting_preference": "auto",`, "", 1)
	h := newNetworkHarness(t, doc)

	cfg := networkBaseConfig()
	cfg["ipv6"] = types.ObjectValueMust(networkIPv6AttrTypes(), map[string]attr.Value{
		"interface_type":            types.StringValue("none"),
		"client_address_assignment": types.StringNull(),
		"static_subnet":             types.StringNull(),
		"setting_preference":        types.StringNull(),
		"aliases":                   types.ListNull(types.StringType),
		"ra":                        types.ObjectNull(networkIPv6RAAttrTypes()),
		"pd":                        types.ObjectNull(networkIPv6PDAttrTypes()),
	})
	h.assertEqual("plan after import", h.state.Raw, h.plan(h.config(cfg)))

	cfg["igmp_snooping"] = types.BoolValue(true)
	planned := plannedObject(t, h.plan(h.config(cfg)), "planned state")
	got := plannedObject(t, planned["ipv6"], "ipv6")["setting_preference"]
	if got.IsKnown() {
		t.Errorf("update planned ipv6.setting_preference = %v, want unknown", got)
	}
	h.apply(h.config(cfg))
}

// A WAN stored without setting_preference reads it as null. An update must
// leave the null prior unknown rather than plan it as a known null, so the
// apply takes whatever the controller holds afterwards.
func TestWANUpdatePlansUnknownForNullSettingPreference(t *testing.T) {
	h := newWANHarness(t)
	h.assertEqual("plan after import", h.state.Raw, h.plan(h.config(wanBaseConfig())))

	cfg := wanBaseConfig()
	cfg["report_wan_event"] = types.BoolValue(true)
	got := plannedObject(t, h.plan(h.config(cfg)), "planned state")["setting_preference"]
	if got.IsKnown() {
		t.Errorf("update planned setting_preference = %v, want unknown", got)
	}
	assertBodies(t, h.apply(h.config(cfg)), `{"report_wan_event":true}`)
}
