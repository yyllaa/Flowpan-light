package main

import (
	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
)

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
	var out map[string]any
	err = pluginwasm.CallCapability("formatter", v.Request.Input, &out)
	if err != nil {
		pluginwasm.Return(pluginapi.Result{Failed: true, Message: err.Error()})
		return
	}
	pluginwasm.Return(pluginapi.Result{Data: out})
}
