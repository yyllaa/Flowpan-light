package pluginapi

import (
	"encoding/json"
	"testing"
)

func TestPluginNetworkDeclaration(t *testing.T) {
	for _, tc := range []struct {
		mode, configured, want string
		invalid                bool
	}{
		{"", "http://proxy:7890", "http://proxy:7890", false},
		{"", "", "", false},
		{ProxyInherit, "http://proxy:7890", "http://proxy:7890", false},
		{ProxyDirect, "http://proxy:7890", "", false},
		{ProxyDirect, "", "", false},
		{ProxyProject, " http://proxy:7890 ", "http://proxy:7890", false},
		{ProxyProject, "  ", "", true},
		{"http://guest-proxy:1234", "http://proxy:7890", "", true},
		{"PROJECT", "http://proxy:7890", "", true},
	} {
		t.Run(tc.mode+"/"+tc.configured, func(t *testing.T) {
			d := scheduleDefinition()
			if tc.mode != "" {
				d.Network = &NetworkPolicy{Proxy: tc.mode}
			}
			got, err := d.ProxyURL(tc.configured)
			if (err != nil) != tc.invalid || got != tc.want {
				t.Fatal(got, err)
			}
			if err := d.Validate(); err != nil && tc.mode != "PROJECT" && tc.mode != "http://guest-proxy:1234" {
				t.Fatal("valid declaration rejected", err)
			}
		})
	}
	for _, raw := range []string{`{"network":{}}`, `{"network":{"proxy":"unknown"}}`, `{"network":{"proxy":true}}`} {
		d := scheduleDefinition()
		if err := json.Unmarshal([]byte(raw), &d); err == nil && d.Validate() == nil {
			t.Fatal("invalid declaration accepted", raw)
		}
	}
}
