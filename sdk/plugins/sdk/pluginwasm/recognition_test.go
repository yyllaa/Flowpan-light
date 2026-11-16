package pluginwasm

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"flowpan/app/pluginapi"
)

func TestPluginAdaptiveRecognitionLifecycle(t *testing.T) {
	for _, mode := range []string{"health", "migrate", "recognition"} {
		calls := 0
		inv := Invocation{ABIVersion: "1", Mode: mode, Request: pluginapi.Request{Recognition: &pluginapi.RecognitionCall{Provider: "catalog", File: pluginapi.RecognitionInput{Name: "NX-001.mkv"}}}}
		b, _ := json.Marshal(inv)
		var out bytes.Buffer
		err := serve(bytes.NewReader(b), &out, Handlers{Run: func(Invocation) (pluginapi.Result, error) {
			t.Fatal("ordinary action ran")
			return pluginapi.Result{}, nil
		}, Recognition: func(v Invocation) (pluginapi.Result, error) {
			calls++
			if _, err := RecognitionContext(v); err != nil {
				t.Fatal(err)
			}
			return RecognitionReply(pluginapi.RecognitionResult{Status: "no_match"})
		}})
		if err != nil || mode == "recognition" && calls != 1 || mode != "recognition" && calls != 0 {
			t.Fatal(mode, calls, err)
		}
	}
	var out bytes.Buffer
	_ = serve(strings.NewReader(`{"abi_version":"1","mode":"recognition"}`), &out, Handlers{})
	var result pluginapi.Result
	_ = json.Unmarshal(out.Bytes(), &result)
	if !result.Failed {
		t.Fatal("missing recognition context accepted")
	}
}
