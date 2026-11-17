// This public example uses only the source SDK, never host internals.
package main

import (
	"encoding/json"
	"fmt"

	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
)

func main() { pluginwasm.Serve(pluginwasm.Handlers{Run: run}) }

func run(i pluginwasm.Invocation) (pluginapi.Result, error) {
	switch i.Request.ActionID {
	case "run":
		contract, err := pluginwasm.Capabilities()
		if err != nil {
			return pluginapi.Result{}, err
		}
		return pluginapi.Result{Message: "已连接项目能力", Data: map[string]any{"capabilities": contract}}, nil
	case "remux":
		var input struct {
			Key    string
			Path   string
			Format string
			Title  string
			Artist string
			Album  string
		}
		b, err := json.Marshal(i.Request.Input)
		if err != nil {
			return pluginapi.Result{}, err
		}
		if err = json.Unmarshal(b, &input); err != nil {
			return pluginapi.Result{}, err
		}
		task, err := pluginwasm.SubmitMedia(pluginapi.MediaProcess{Key: input.Key, Path: input.Path, Operation: "remux", Format: input.Format, Metadata: pluginapi.MediaMetadata{Title: input.Title, Artist: input.Artist, Album: input.Album}})
		if err != nil {
			return pluginapi.Result{}, err
		}
		return pluginapi.Result{Message: "已提交音乐封装", Data: map[string]any{"media": task}}, nil
	case "status":
		id, _ := i.Request.Input["id"].(string)
		task, err := pluginwasm.MediaTask(id)
		if err != nil {
			return pluginapi.Result{}, err
		}
		return pluginapi.Result{Message: "媒体任务状态", Data: map[string]any{"media": task}}, nil
	default:
		return pluginapi.Result{}, fmt.Errorf("动作未声明")
	}
}
