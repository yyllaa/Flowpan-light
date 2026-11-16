package main

import (
	"encoding/json"
	"fmt"

	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
)

func main() { pluginwasm.Serve(pluginwasm.Handlers{Run: run}) }
func run(inv pluginwasm.Invocation) (pluginapi.Result, error) {
	r := inv.Request
	switch r.ActionID {
	case "generate":
		var input struct {
			Account string   `json:"account"`
			IDs     []string `json:"ids"`
			ID      string   `json:"id"`
			FileID  string   `json:"file_id_text"`
		}
		b, err := json.Marshal(r.Input)
		if err != nil {
			return pluginapi.Result{}, err
		}
		if err = json.Unmarshal(b, &input); err != nil {
			return pluginapi.Result{}, err
		}
		if len(input.IDs) == 0 {
			if input.ID != "" {
				input.IDs = []string{input.ID}
			} else if input.FileID != "" {
				input.IDs = []string{input.FileID}
			}
		}
		if input.Account == "" || len(input.IDs) == 0 || len(input.IDs) > 50 {
			return pluginapi.Result{}, fmt.Errorf("请在 115 文件浏览中选择 1–50 个文件")
		}
		var accounts []struct {
			ID       string `json:"id"`
			Provider string `json:"provider"`
		}
		if err = pluginwasm.Accounts(&accounts); err != nil {
			return pluginapi.Result{}, err
		}
		supported := false
		for _, a := range accounts {
			supported = supported || a.ID == input.Account && a.Provider == "p115"
		}
		if !supported {
			return pluginapi.Result{}, fmt.Errorf("请选择已授权的 115 账号")
		}
		items := []pluginapi.ED2KTask{}
		failures := []map[string]string{}
		seen := map[string]bool{}
		for _, id := range input.IDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			task, err := pluginwasm.GenerateED2K(input.Account, id)
			if err != nil {
				failures = append(failures, map[string]string{"id": id, "error": err.Error()})
				continue
			}
			items = append(items, task)
		}
		return pluginapi.Result{Message: fmt.Sprintf("已提交 %d 个文件，%d 个失败；在 ED2K 管理中查看进度", len(items), len(failures)), Failed: len(items) == 0, Data: map[string]any{"items": items, "failures": failures}}, nil
	case "list":
		var query pluginapi.ED2KQuery
		b, _ := json.Marshal(r.Input)
		if err := json.Unmarshal(b, &query); err != nil {
			return pluginapi.Result{}, err
		}
		page, err := pluginwasm.ListED2K(query)
		if err != nil {
			return pluginapi.Result{}, err
		}
		var accounts any
		if err = pluginwasm.Accounts(&accounts); err != nil {
			return pluginapi.Result{}, err
		}
		return pluginapi.Result{Data: map[string]any{"page": page, "accounts": accounts}}, nil
	case "task", "retry", "cancel":
		id, _ := r.Input["id"].(string)
		var task pluginapi.ED2KTask
		var err error
		switch r.ActionID {
		case "task":
			task, err = pluginwasm.ED2KTask(id)
		case "retry":
			task, err = pluginwasm.RetryED2K(id)
		case "cancel":
			task, err = pluginwasm.CancelED2K(id)
		}
		return pluginapi.Result{Message: task.Message, Data: map[string]any{"task": task}}, err
	case "remove":
		id, _ := r.Input["id"].(string)
		if err := pluginwasm.RemoveED2K(id); err != nil {
			return pluginapi.Result{}, err
		}
		return pluginapi.Result{Message: "ED2K 记录已删除"}, nil
	}
	return pluginapi.Result{}, fmt.Errorf("动作未声明")
}
