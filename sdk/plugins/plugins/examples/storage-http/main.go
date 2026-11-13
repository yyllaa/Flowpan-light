// This starter adapts an HTTPS JSON storage API. Replace the forwarding here
// with the target provider's protocol; no host application code is required.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
)

func main() { pluginwasm.Serve(pluginwasm.Handlers{Storage: storageCall}) }
func storageCall(i pluginwasm.Invocation) (pluginapi.Result, error) {
	call := i.Request.Storage
	if call == nil {
		return pluginapi.Result{}, fmt.Errorf("缺少网盘调用")
	}
	// Dynamic choices are a read-only protocol operation, not plugin page code.
	if call.Operation == "ui.options" && call.Params.Field == "client_version" {
		return pluginwasm.StorageResult([]pluginapi.Option{{Value: "v1", Label: "版本一"}, {Value: "v2", Label: "版本二"}}), nil
	}
	endpoint, _ := call.Settings["endpoint"].(string)
	token, _ := call.Settings["token"].(string)
	var credentials struct {
		Token string `json:"token"`
	}
	if len(call.Credentials) > 0 {
		if err := json.Unmarshal(call.Credentials, &credentials); err != nil {
			return pluginapi.Result{}, err
		}
	}
	if credentials.Token != "" && call.Operation != "configure" && call.Operation != "login.begin" {
		token = credentials.Token
	}
	raw, err := json.Marshal(call)
	if err != nil {
		return pluginapi.Result{}, err
	}
	var response struct {
		Status int    `json:"status"`
		Body   string `json:"body"`
	}
	err = pluginwasm.Call("http.fetch", map[string]any{"url": strings.TrimRight(endpoint, "/") + "/storage", "method": "POST", "headers": map[string]string{"Authorization": "Bearer " + token, "Content-Type": "application/json"}, "body": string(raw)}, &response)
	if err != nil {
		return pluginapi.Result{}, err
	}
	if response.Status != 200 {
		return pluginapi.Result{}, fmt.Errorf("网盘接口返回 HTTP %d", response.Status)
	}
	var result any
	if err = json.Unmarshal([]byte(response.Body), &result); err != nil {
		return pluginapi.Result{}, fmt.Errorf("网盘接口结果无效")
	}
	return pluginwasm.StorageResult(result), nil
}
