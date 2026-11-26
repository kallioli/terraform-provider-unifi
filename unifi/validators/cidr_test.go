package validators

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCIDRValidator(t *testing.T) {
	v := CIDRValidator()

	cases := []struct {
		name  string
		value types.String
		valid bool
	}{
		{"IPv4 network", types.StringValue("192.168.1.0/24"), true},
		{"IPv4 host route", types.StringValue("10.0.0.1/32"), true},
		{"IPv4 default route", types.StringValue("0.0.0.0/0"), true},
		// net.ParseCIDR accepts host bits; the validator only checks syntax.
		{"IPv4 host bits set", types.StringValue("10.0.0.1/24"), true},
		{"IPv6 network", types.StringValue("2001:db8::/32"), true},
		{"IPv6 host route", types.StringValue("2001:db8::1/128"), true},
		{"missing prefix length", types.StringValue("192.168.1.0"), false},
		{"IPv4 prefix too long", types.StringValue("192.168.1.0/33"), false},
		{"IPv6 prefix too long", types.StringValue("2001:db8::/129"), false},
		{"invalid octet", types.StringValue("192.168.1.256/24"), false},
		{"garbage", types.StringValue("not-a-cidr"), false},
		{"empty", types.StringValue(""), false},
		{"null skips", types.StringNull(), true},
		{"unknown skips", types.StringUnknown(), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			v.ValidateString(context.Background(), validator.StringRequest{
				Path:        path.Root("network"),
				ConfigValue: tc.value,
			}, resp)

			got := !resp.Diagnostics.HasError()
			if got != tc.valid {
				t.Fatalf("got valid=%v, want %v for %q: %v",
					got, tc.valid, tc.value.ValueString(), resp.Diagnostics)
			}
			if tc.valid {
				return
			}

			diag := resp.Diagnostics.Errors()[0]
			if diag.Summary() != "Invalid CIDR Notation" {
				t.Errorf("summary = %q, want %q", diag.Summary(), "Invalid CIDR Notation")
			}
			if !strings.Contains(diag.Detail(), `"`+tc.value.ValueString()+`"`) {
				t.Errorf("detail %q does not quote the rejected value", diag.Detail())
			}
		})
	}
}

func Test_cidrValidator_Description(t *testing.T) {
	want := "value must be a valid CIDR notation"
	v := cidrValidator{}

	if got := v.Description(context.Background()); got != want {
		t.Errorf("Description() = %q, want %q", got, want)
	}
	if got := v.MarkdownDescription(context.Background()); got != want {
		t.Errorf("MarkdownDescription() = %q, want %q", got, want)
	}
}
