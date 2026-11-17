package pluginapi

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDeclarativeRequirements(t *testing.T) {
	h := CurrentHostContract("2026.11.17-contract")
	required := &HostRequirements{APIVersion: "1", Features: map[string]int{"jobs.continuation": 1}, Tools: []string{"files.list"}, Limits: map[string]int64{"host_calls": 128}}
	optional := &HostRequirements{Features: map[string]int{"future.cover-picker": 1}, Tools: []string{"future.tool"}, Limits: map[string]int64{"memory_mib": 1024}}
	r := h.Check(required, optional, "")
	if !r.Compatible || r.Err() != nil || len(r.OptionalMissing) != 3 {
		t.Fatalf("optional dependency blocks ordinary updates: %+v", r)
	}
	required.Features["jobs.continuation"] = 2
	required.Tools = append(required.Tools, "future.required")
	required.Limits["host_calls"] = 129
	r = h.Check(required, optional, "")
	if r.Compatible || r.Err() == nil || len(r.Missing) != 3 {
		t.Fatalf("incomplete missing report: %+v", r)
	}
	for _, bad := range []*HostRequirements{{Features: map[string]int{"bad key": 1}}, {Features: map[string]int{"ok": 0}}, {Tools: []string{"files.list", "files.list"}}, {Limits: map[string]int64{"memory_mib": 0}}} {
		if bad.Validate() == nil {
			t.Fatalf("invalid declaration accepted: %+v", bad)
		}
	}
	t.Log("DECLARATIVE_REQUIREMENTS_PASS required=blocked optional=degraded tools=listed limits=checked")
}

func TestMinimumHostVersionCompatibility(t *testing.T) {
	for _, tc := range []struct {
		actual, min string
		want        bool
	}{
		{"2026.11.17-contract", "2026.11.16", true}, {"2026.11.17-contract", "2026.11.18", false},
		{"1.10.0", "1.9", true}, {"v1.2.0+local", "1.2", true}, {"core-dev", "2026.11.17", false},
	} {
		if got := hostVersionAtLeast(tc.actual, tc.min); got != tc.want {
			t.Errorf("%s >= %s: %v", tc.actual, tc.min, got)
		}
	}
	if !CurrentHostContract("core-dev").Check(nil, nil, "1").Compatible {
		t.Fatal("legacy API alias rejected")
	}
}

func TestNamespacedExtensionsPreserveUnknownData(t *testing.T) {
	a := Extensions{"vendor.music": json.RawMessage(`{ "z": [1,"中文"], "a":true }`)}
	b := Extensions{"vendor.music": json.RawMessage(`{"a":true,"z":[1,"中文"]}`)}
	if a.Normalize() != nil || b.Normalize() != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("extension canonicalization: %s %s", a, b)
	}
	for _, bad := range []Extensions{{"unscoped": json.RawMessage(`true`)}, {"vendor.bad": json.RawMessage(`{} {}`)}, {"vendor.big": json.RawMessage(`"` + string(make([]byte, 65536)) + `"`)}} {
		if bad.Normalize() == nil {
			t.Fatal("invalid extension accepted")
		}
	}
}
