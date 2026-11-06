package pluginwasm

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"flowpan/app/pluginapi"
)

func TestServeLifecycleAndFailure(t *testing.T) {
	for _, mode := range []string{"health", "migrate", "run", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			var output bytes.Buffer
			if err := serve(strings.NewReader(`{"abi_version":"1","mode":"`+mode+`"}`), &output, Handlers{Run: func(Invocation) (pluginapi.Result, error) { calls++; return pluginapi.Result{Message: "运行"}, nil }}); err != nil {
				t.Fatal(err)
			}
			var result pluginapi.Result
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if (mode == "run") != (calls == 1) || result.Failed != (mode == "invalid") {
				t.Fatal(mode, calls, result)
			}
		})
	}
	for _, input := range []string{`{`, `{"abi_version":"9","mode":"run"}`, `{"abi_version":"1","mode":"run"} {}`} {
		var output bytes.Buffer
		_ = serve(strings.NewReader(input), &output, Handlers{Run: func(Invocation) (pluginapi.Result, error) {
			t.Fatal("invalid invocation executed")
			return pluginapi.Result{}, nil
		}})
		var result pluginapi.Result
		_ = json.Unmarshal(output.Bytes(), &result)
		if !result.Failed {
			t.Fatal("invalid input accepted")
		}
	}
	for _, panicHandler := range []bool{false, true} {
		var output bytes.Buffer
		_ = serve(strings.NewReader(`{"abi_version":"1","mode":"run"}`), &output, Handlers{Run: func(Invocation) (pluginapi.Result, error) {
			if panicHandler {
				panic("private details")
			}
			return pluginapi.Result{}, errors.New("业务失败")
		}})
		var result pluginapi.Result
		_ = json.Unmarshal(output.Bytes(), &result)
		if !result.Failed || strings.Contains(result.Message, "private details") {
			t.Fatal("failure not handled", result)
		}
	}
}
