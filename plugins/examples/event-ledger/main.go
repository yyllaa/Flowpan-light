package main

import (
	"encoding/json"
	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
)

type Ledger struct {
	Events []string `json:"events"`
	Count  int      `json:"count"`
}

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
	// Development probes exercise actual guest execution, cancellation and limits.
	if v.Request.Input["test"] == "spin" {
		for {
		}
	}
	if v.Request.Input["test"] == "memory" {
		b := make([]byte, 80<<20)
		for j := range b {
			b[j] = byte(j)
		}
		pluginwasm.Return(pluginapi.Result{Message: string(b[:10])})
		return
	}
	if method, ok := v.Request.Input["method"].(string); ok {
		e = pluginwasm.Call(method, v.Request.Input["params"], nil)
		if e != nil {
			pluginwasm.Return(pluginapi.Result{Message: e.Error(), Failed: true})
			return
		}
	}
	var ledger Ledger
	if e = pluginwasm.Call("kv.get", map[string]any{"key": "ledger"}, &ledger); e != nil {
		pluginwasm.Return(pluginapi.Result{Message: e.Error(), Failed: true})
		return
	}
	eventID := v.Request.TaskID
	if v.Request.Event != nil {
		eventID = v.Request.Event.ID
	}
	for _, id := range ledger.Events {
		if id == eventID {
			pluginwasm.Return(pluginapi.Result{Message: "事件已处理", Data: map[string]any{"count": ledger.Count}})
			return
		}
	}
	ledger.Count++
	ledger.Events = append(ledger.Events, eventID)
	if len(ledger.Events) > 100 {
		ledger.Events = ledger.Events[len(ledger.Events)-100:]
	}
	_ = pluginwasm.Call("task.progress", pluginapi.Progress{Done: 1, Total: 1, Message: "已记录"}, nil)
	b, _ := json.Marshal(ledger)
	if e = pluginwasm.Call("kv.set", map[string]any{"key": "ledger", "value": json.RawMessage(b)}, nil); e != nil {
		pluginwasm.Return(pluginapi.Result{Message: e.Error(), Failed: true})
		return
	}
	items := []pluginapi.ResultItem{}
	for _, id := range ledger.Events {
		items = append(items, pluginapi.ResultItem{Label: "事件", Message: id})
	}
	pluginwasm.Return(pluginapi.Result{Message: "事件已记录", Data: map[string]any{"count": ledger.Count}, Items: items})
}
