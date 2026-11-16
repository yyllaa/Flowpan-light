package main

import (
	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
	"time"
)

type host interface{ Call(string, any, any) error }
type wasmHost struct{}

func (wasmHost) Call(method string, input, output any) error {
	return pluginwasm.Call(method, input, output)
}

func main() {
	pluginwasm.Serve(pluginwasm.Handlers{Run: func(i pluginwasm.Invocation) (pluginapi.Result, error) {
		return (&engine{host: wasmHost{}, now: time.Now}).invoke(i.Request)
	}, Health: func(pluginwasm.Invocation) (pluginapi.Result, error) {
		return pluginapi.Result{Message: "音乐刮削已就绪"}, nil
	}, Migrate: func(pluginwasm.Invocation) (pluginapi.Result, error) {
		return pluginapi.Result{Message: "音乐刮削数据已就绪"}, nil
	}})
}
