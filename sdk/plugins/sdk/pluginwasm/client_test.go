package pluginwasm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"flowpan/app/pluginapi"
)

func TestCallCompatibilityAndStructuredErrors(t *testing.T) {
	for _, response := range []string{`{"error":"工具停用","error_code":"tool_disabled"}`, `{"error":"旧版错误"}`} {
		err := callHost(func(request, out []byte) uint32 {
			var call map[string]json.RawMessage
			if json.Unmarshal(request, &call) != nil || string(call["params"]) != "{}" {
				t.Fatal("nil parameters must be an object")
			}
			return uint32(copy(out, response))
		}, "files.list", nil, nil)
		var detail *pluginapi.HostError
		if !errors.As(err, &detail) || detail.Method != "files.list" {
			t.Fatal("missing structured error", err)
		}
		want := "tool_disabled"
		if strings.Contains(response, "旧版") {
			want = "host_error"
		}
		if ErrorCode(fmt.Errorf("context: %w", err)) != want {
			t.Fatal("wrapped code lost", err)
		}
	}
	var data struct {
		Value int `json:"value"`
	}
	if err := callHost(func(_, out []byte) uint32 { return uint32(copy(out, `{"data":{"value":7}}`)) }, "kv.get", map[string]any{"key": "test"}, &data); err != nil || data.Value != 7 {
		t.Fatal(data, err)
	}
	calls := 0
	if err := callHost(func(_, out []byte) uint32 { calls++; return 0 }, "log", map[string]any{"message": strings.Repeat("x", pluginapi.MaxHostRequest)}, nil); err == nil || calls != 0 {
		t.Fatal("oversize request reached host")
	}
}
