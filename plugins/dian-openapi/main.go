package main

import (
	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
)

type host interface{ Call(string, any, any) error }
type wasmHost struct{}

func (wasmHost) Call(method string, p any, out any) error { return pluginwasm.Call(method, p, out) }

type receipt struct {
	Until   int64            `json:"until"`
	Pending bool             `json:"pending,omitempty"`
	Result  pluginapi.Result `json:"result"`
}

func main() {
	inv, e := pluginwasm.Read()
	if e != nil {
		pluginwasm.Return(pluginapi.Result{Message: "插件输入格式错误", Failed: true})
		return
	}
	if inv.Mode == "health" || inv.Mode == "migrate" {
		pluginwasm.Return(pluginapi.Result{Message: "癫影插件已就绪"})
		return
	}
	result, e := executeOpenAPI(wasmHost{}, inv.Request)
	if e != nil {
		result = pluginapi.Result{Message: e.Error(), Failed: true}
	}
	pluginwasm.Return(result)
}
