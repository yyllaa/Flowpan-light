package main

import (
	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
)

func main() { pluginwasm.Serve(pluginwasm.Handlers{Run: run}) }

func run(invocation pluginwasm.Invocation) (pluginapi.Result, error) {
	contract, err := pluginwasm.Capabilities()
	if err != nil {
		return pluginapi.Result{}, err
	}
	if err = pluginwasm.ReportProgress(pluginapi.Progress{Done: 1, Total: 2, Message: "已连接宿主"}); err != nil {
		return pluginapi.Result{}, err
	}
	text, _ := invocation.Request.Config["message"].(string)
	if text == "" {
		text = "外部插件已接入 Flowpan"
	}
	if err = pluginwasm.Log(text); err != nil {
		return pluginapi.Result{}, err
	}
	if err = pluginwasm.Set("last-task", invocation.Request.TaskID); err != nil {
		return pluginapi.Result{}, err
	}
	var saved string
	if err = pluginwasm.Get("last-task", &saved); err != nil {
		return pluginapi.Result{}, err
	}
	if err = pluginwasm.ReportProgress(pluginapi.Progress{Done: 2, Total: 2, Message: "接入检查完成"}); err != nil {
		return pluginapi.Result{}, err
	}
	return pluginapi.Result{Message: text, Data: map[string]any{"api_version": contract.APIVersion, "tools": len(contract.Tools), "checkpoint_saved": saved == invocation.Request.TaskID}}, nil
}
