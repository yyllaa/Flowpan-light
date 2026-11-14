// This example separates cloud transport from media matching. Each invocation
// submits or queries one task, so a large cloud file never consumes WASM memory
// or requires polling past the host-call budget in a single invocation.
package main

import (
	"encoding/json"
	"fmt"

	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
)

func main() { pluginwasm.Serve(pluginwasm.Handlers{Run: run}) }
func run(i pluginwasm.Invocation) (pluginapi.Result, error) {
	var input struct {
		Key       string `json:"key"`
		Account   string `json:"account"`
		ID        string `json:"id"`
		ParentID  string `json:"parent_id"`
		Name      string `json:"name"`
		FileTask  string `json:"file_task"`
		MediaTask string `json:"media_task"`
		Conflict  string `json:"conflict"`
		Format    string `json:"format"`
		Title     string `json:"title"`
		Artist    string `json:"artist"`
		Album     string `json:"album"`
	}
	data, _ := json.Marshal(i.Request.Input)
	if err := json.Unmarshal(data, &input); err != nil {
		return pluginapi.Result{}, err
	}
	result := func(value any, err error) (pluginapi.Result, error) {
		if err != nil {
			return pluginapi.Result{}, err
		}
		return pluginapi.Result{Data: map[string]any{"task": value}}, nil
	}
	switch i.Request.ActionID {
	case "run":
		var status any
		if err := pluginwasm.Call("media.status", map[string]any{}, &status); err != nil {
			return pluginapi.Result{}, err
		}
		caps, err := pluginwasm.FileCapabilities(input.Account)
		if err != nil {
			return pluginapi.Result{}, err
		}
		return pluginapi.Result{Data: map[string]any{"media": status, "files": caps}}, nil
	case "stage":
		return result(pluginwasm.StageFile(pluginapi.FileStage{Key: input.Key, Account: input.Account, ID: input.ID}))
	case "probe":
		var metadata any
		err := pluginwasm.ProbeStagedMedia(input.FileTask, &metadata)
		return result(metadata, err)
	case "process":
		return result(pluginwasm.SubmitMedia(pluginapi.MediaProcess{Key: input.Key, FileTask: input.FileTask, Operation: "remux", Format: input.Format, Metadata: pluginapi.MediaMetadata{Title: input.Title, Artist: input.Artist, Album: input.Album}}))
	case "publish":
		original := ""
		if input.Conflict == "replace" {
			original = input.FileTask
		}
		return result(pluginwasm.PublishFile(pluginapi.FilePublish{Key: input.Key, Account: input.Account, ParentID: input.ParentID, Name: input.Name, MediaTask: input.MediaTask, OriginalTask: original, Conflict: input.Conflict}))
	case "status":
		return result(pluginwasm.FileTask(input.ID))
	case "media-status":
		return result(pluginwasm.MediaTask(input.ID))
	case "cancel":
		return result(pluginwasm.CancelFile(input.ID))
	case "restore":
		return result(pluginwasm.RestoreFile(input.ID, input.Key))
	case "release-file":
		return result(pluginwasm.ReleaseFile(input.ID))
	case "release-media":
		return result(pluginwasm.ReleaseMedia(input.ID))
	}
	return pluginapi.Result{}, fmt.Errorf("未知文件处理动作")
}
