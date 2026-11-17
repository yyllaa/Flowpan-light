package main

import (
	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
)

func main() {
	v, e := pluginwasm.Read()
	if e != nil {
		pluginwasm.Return(pluginapi.Result{Message: e.Error(), Failed: true})
		return
	}
	if v.Mode != "run" {
		pluginwasm.Return(pluginapi.Result{Message: "ready"})
		return
	}
	text, _ := v.Request.Config["message"].(string)
	if text == "" {
		text = "你好，Flowpan"
	}
	if e = pluginwasm.Call("log", map[string]any{"message": text}, nil); e != nil {
		pluginwasm.Return(pluginapi.Result{Message: e.Error(), Failed: true})
		return
	}
	pluginwasm.Return(pluginapi.Result{Message: text, Data: map[string]any{"completed": 1}})
}
