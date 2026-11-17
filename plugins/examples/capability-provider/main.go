package main

import (
	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
	"strings"
)

// This algorithm is owned by the provider. The host knows only its versioned
// interface, schemas and action mapping, not the implementation or plugin ID.
func main() {
	v, err := pluginwasm.Read()
	if err != nil {
		pluginwasm.Return(pluginapi.Result{Failed: true, Message: err.Error()})
		return
	}
	if v.Mode != "run" {
		pluginwasm.Return(pluginapi.Result{Message: "ready"})
		return
	}
	text, _ := v.Request.Input["text"].(string)
	pluginwasm.Return(pluginapi.Result{Data: map[string]any{"text": strings.ToUpper(strings.TrimSpace(text))}})
}
