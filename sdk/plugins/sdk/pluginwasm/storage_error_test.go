package pluginwasm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"flowpan/app/pluginapi"
)

func TestPluginAdaptiveStorageErrorWire(t *testing.T) {
	for _, mode := range []string{"storage", "run"} {
		var output bytes.Buffer
		handler := func(Invocation) (pluginapi.Result, error) {
			return pluginapi.Result{}, fmt.Errorf("wrapped: %w", &pluginapi.StorageError{Code: "AUTH_EXPIRED", Status: 401, Message: "Login expired"})
		}
		input := `{"abi_version":"1","mode":"` + mode + `","request":{"storage":{"operation":"link","account":"account-a"}}}`
		if err := serve(strings.NewReader(input), &output, Handlers{Storage: handler, Run: handler}); err != nil {
			t.Fatal(err)
		}
		var result pluginapi.Result
		if err := json.Unmarshal(output.Bytes(), &result); err != nil || !result.Failed {
			t.Fatal(result, err)
		}
		wire, ok := result.Data["storage_error"]
		if ok != (mode == "storage") {
			t.Fatal("storage failure escaped its lifecycle", mode)
		}
		if ok && wire.(map[string]any)["code"] != "AUTH_EXPIRED" {
			t.Fatal("business code lost", wire)
		}
	}
}
