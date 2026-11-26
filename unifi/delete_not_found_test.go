package unifi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/ubiquiti-community/go-unifi/unifi"
)

// cannedReply is one HTTP answer of the delete fake controller.
type cannedReply struct {
	status int
	body   string
}

const (
	restEmpty     = `{"meta":{"rc":"ok"},"data":[]}`
	restIDInvalid = `{"meta":{"rc":"error","msg":"api.err.IdInvalid"},"data":[]}`
)

// newDeleteFakeController serves the canned replies keyed by "METHOD /path",
// and fails the test on any other request. It records the keys it served.
func newDeleteFakeController(
	t *testing.T,
	replies map[string]cannedReply,
) (*Client, func() []string) {
	t.Helper()
	var (
		mu   sync.Mutex
		seen []string
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/manage", http.StatusFound)
			return
		}
		if r.URL.Path == "/status" { // probed while the client is set up
			http.NotFound(w, r)
			return
		}
		key := r.Method + " " + r.URL.Path
		reply, ok := replies[key]
		if !ok {
			t.Errorf("unexpected request %s", key)
			w.WriteHeader(http.StatusTeapot)
			return
		}
		mu.Lock()
		seen = append(seen, key)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(reply.status)
		_, _ = w.Write([]byte(reply.body))
	})
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "unifises", Value: "fake-session", Path: "/"})
		_, _ = w.Write([]byte(restEmpty))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	apiClient, err := unifi.New(context.Background(), &unifi.Config{
		BaseURL: srv.URL, Username: "admin", Password: "admin",
	})
	if err != nil {
		t.Fatalf("creating client against fake controller: %v", err)
	}
	return &Client{ApiClient: apiClient, Site: "default"}, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
}

// TestDeleteAlreadyGone checks that destroying an object that is no longer on
// the controller succeeds, whether the delete answers 404 (v2 endpoints) or
// another error that a fresh lookup shows to mean "gone" (legacy REST and
// sitemgr, whose answer for an unknown ID is not documented). A delete that
// fails while the object is still there must keep failing.
func TestDeleteAlreadyGone(t *testing.T) {
	cases := []struct {
		name     string
		resource func(*Client) fwresource.Resource
		state    map[string]any
		del      string // request key of the delete
		get      string // request key of the lookup
		gone     string // lookup body when the object is gone
		gone404  bool   // the lookup answers 404 when the object is gone
		present  string // lookup body when the object is still there
	}{
		{
			name:     "dns_record",
			resource: func(c *Client) fwresource.Resource { return &dnsRecordFrameworkResource{client: c} },
			state:    map[string]any{"id": "dns1", "site": "default", "name": "h.example.com"},
			del:      "DELETE /v2/api/site/default/static-dns/dns1",
			get:      "GET /v2/api/site/default/static-dns",
			gone:     `[]`,
			present:  `[{"_id":"dns1","key":"h.example.com","record_type":"A"}]`,
		},
		{
			name:     "firewall_zone",
			resource: func(c *Client) fwresource.Resource { return &firewallZoneResource{client: c} },
			state:    map[string]any{"id": "z1", "site": "default", "name": "zone"},
			del:      "DELETE /v2/api/site/default/firewall/zone/z1",
			get:      "GET /v2/api/site/default/firewall/zone",
			gone:     `[{"_id":"z2","name":"other"}]`,
			present:  `[{"_id":"z1","name":"zone"}]`,
		},
		{
			name:     "network",
			resource: func(c *Client) fwresource.Resource { return &networkResource{client: c} },
			state:    map[string]any{"id": "net1", "site": "default", "name": "LAN"},
			del:      "DELETE /api/s/default/rest/networkconf/net1",
			get:      "GET /api/s/default/rest/networkconf/net1",
			gone:     restEmpty,
			present:  `{"meta":{"rc":"ok"},"data":[{"_id":"net1","name":"LAN"}]}`,
		},
		{
			name:     "radius_user",
			resource: func(c *Client) fwresource.Resource { return &radiusUserResource{client: c} },
			state:    map[string]any{"id": "acc1", "site": "default", "name": "user"},
			del:      "DELETE /api/s/default/rest/account/acc1",
			get:      "GET /api/s/default/rest/account/acc1",
			gone:     restEmpty,
			present:  `{"meta":{"rc":"ok"},"data":[{"_id":"acc1","name":"user"}]}`,
		},
		{
			name:     "site",
			resource: func(c *Client) fwresource.Resource { return &siteFrameworkResource{client: c} },
			state:    map[string]any{"id": "site1", "name": "branch"},
			del:      "POST /api/s/default/cmd/sitemgr",
			get:      "GET /api/self/sites",
			gone:     `{"meta":{"rc":"ok"},"data":[{"_id":"site0","name":"default"}]}`,
			present:  `{"meta":{"rc":"ok"},"data":[{"_id":"site1","name":"branch"}]}`,
		},
		{
			name:     "vpn_client",
			resource: func(c *Client) fwresource.Resource { return &vpnClientResource{client: c} },
			state:    map[string]any{"id": "net1", "site": "default", "name": "vpn"},
			del:      "DELETE /api/s/default/rest/networkconf/net1",
			get:      "GET /api/s/default/rest/networkconf/net1",
			gone:     restEmpty,
			present:  `{"meta":{"rc":"ok"},"data":[{"_id":"net1","name":"vpn"}]}`,
		},
		{
			name:     "vpn_server",
			resource: func(c *Client) fwresource.Resource { return &vpnServerResource{client: c} },
			state:    map[string]any{"id": "net1", "site": "default", "name": "vpn"},
			del:      "DELETE /api/s/default/rest/networkconf/net1",
			get:      "GET /api/s/default/rest/networkconf/net1",
			gone:     restEmpty,
			present:  `{"meta":{"rc":"ok"},"data":[{"_id":"net1","name":"vpn"}]}`,
		},
		{
			name:     "wan",
			resource: func(c *Client) fwresource.Resource { return &wanResource{client: c} },
			state:    map[string]any{"id": "net1", "site": "default", "name": "wan2"},
			del:      "DELETE /api/s/default/rest/networkconf/net1",
			get:      "GET /api/s/default/rest/networkconf/net1",
			gone:     restEmpty,
			present:  `{"meta":{"rc":"ok"},"data":[{"_id":"net1","name":"wan2"}]}`,
		},
		{
			name: "wireguard_peer",
			resource: func(c *Client) fwresource.Resource {
				return &wireguardPeerResource{client: c}
			},
			state: map[string]any{
				"id": "peer1", "site": "default", "network_id": "srv1", "name": "laptop",
			},
			del:     "POST /v2/api/site/default/wireguard/srv1/users/batch_delete",
			get:     "GET /v2/api/site/default/wireguard/srv1/users",
			gone404: true,
			present: `[{"_id":"peer1","name":"laptop"}]`,
		},
		{
			name:     "wlan",
			resource: func(c *Client) fwresource.Resource { return &wlanFrameworkResource{client: c} },
			state:    map[string]any{"id": "wlan1", "site": "default", "name": "ssid"},
			del:      "DELETE /api/s/default/rest/wlanconf/wlan1",
			get:      "GET /api/s/default/rest/wlanconf/wlan1",
			gone:     restEmpty,
			present:  `{"meta":{"rc":"ok"},"data":[{"_id":"wlan1","name":"ssid"}]}`,
		},
	}

	notFound := cannedReply{http.StatusNotFound, `{}`}
	idInvalid := cannedReply{http.StatusBadRequest, restIDInvalid}

	for _, tc := range cases {
		goneReply := cannedReply{http.StatusOK, tc.gone}
		if tc.gone404 {
			goneReply = notFound
		}
		scenarios := []struct {
			name    string
			replies map[string]cannedReply
			wantErr bool
		}{
			{
				name:    "delete answers 404",
				replies: map[string]cannedReply{tc.del: notFound},
			},
			{
				name:    "delete fails and the object is gone",
				replies: map[string]cannedReply{tc.del: idInvalid, tc.get: goneReply},
			},
			{
				name: "delete fails and the object is still there",
				replies: map[string]cannedReply{
					tc.del: idInvalid,
					tc.get: {http.StatusOK, tc.present},
				},
				wantErr: true,
			},
		}
		for _, sc := range scenarios {
			t.Run(tc.name+"/"+sc.name, func(t *testing.T) {
				client, seen := newDeleteFakeController(t, sc.replies)
				r := tc.resource(client)
				s := resourceSchema(t, r)
				state := tfState(t, s, tc.state)
				resp := &fwresource.DeleteResponse{State: state}
				r.Delete(context.Background(), fwresource.DeleteRequest{State: state}, resp)

				if got := resp.Diagnostics.HasError(); got != sc.wantErr {
					t.Fatalf("error = %v, want %v: %v", got, sc.wantErr, resp.Diagnostics)
				}
				if got := seen(); len(got) == 0 || got[0] != tc.del {
					t.Fatalf("requests = %v, want the delete %q first", got, tc.del)
				}
			})
		}
	}
}

// TestDynamicDNSPrefersStateID checks that unifi_dynamic_dns looks its object
// up by the state ID, not by the stored identity. The identity cannot change
// once stored, so after Read follows the object to a new ID it still holds
// the old one; looking up by it dropped the resource from state on refresh
// and sent the delete to the old ID.
func TestDynamicDNSPrefersStateID(t *testing.T) {
	ctx := context.Background()
	const object = `{"meta":{"rc":"ok"},"data":[{"_id":"new1","service":"dyndns",` +
		`"host_name":"h.example.com","interface":"wan"}]}`
	client, seen := newDeleteFakeController(t, map[string]cannedReply{
		"GET /api/s/default/rest/dynamicdns/new1":    {http.StatusOK, object},
		"GET /api/s/default/rest/dynamicdns/old1":    {http.StatusOK, restEmpty},
		"DELETE /api/s/default/rest/dynamicdns/new1": {http.StatusOK, restEmpty},
	})
	r := &dynamicDNSResource{client: client}
	s := resourceSchema(t, r)
	state := tfState(t, s, map[string]any{
		"id": "new1", "site": "default", "service": "dyndns", "host_name": "h.example.com",
	})
	identity := tfIdentity(t, r)
	identity.Raw = tftypes.NewValue(identity.Raw.Type(), map[string]tftypes.Value{
		"id":   tftypes.NewValue(tftypes.String, "old1"),
		"site": tftypes.NewValue(tftypes.String, "default"),
	})

	readResp := &fwresource.ReadResponse{State: state, Identity: identity}
	r.Read(ctx, fwresource.ReadRequest{State: state, Identity: identity}, readResp)
	if readResp.Diagnostics.HasError() {
		t.Fatalf("read: %v", readResp.Diagnostics)
	}
	if readResp.State.Raw.IsNull() {
		t.Fatalf("read dropped the resource; requests = %v", seen())
	}

	delResp := &fwresource.DeleteResponse{State: state}
	r.Delete(ctx, fwresource.DeleteRequest{State: state, Identity: identity}, delResp)
	if delResp.Diagnostics.HasError() {
		t.Fatalf("delete: %v; requests = %v", delResp.Diagnostics, seen())
	}
	got := seen()
	if want := "DELETE /api/s/default/rest/dynamicdns/new1"; got[len(got)-1] != want {
		t.Fatalf("requests = %v, want the last one to be %q", got, want)
	}
}
