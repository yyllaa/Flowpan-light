// This is a host-contract scaffold, not a metadata or backup implementation.
package main

import (
	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
)

func main() {
	pluginwasm.Serve(pluginwasm.Handlers{Run: func(i pluginwasm.Invocation) (pluginapi.Result, error) {
		instances, err := pluginwasm.EmbyInstances()
		if err != nil {
			return pluginapi.Result{}, err
		}
		data := map[string]any{"instances": instances}
		if instance, _ := i.Request.Input["instance_id"].(string); instance != "" {
			var capabilities any
			if err = pluginwasm.EmbyCapabilities(instance, &capabilities); err != nil {
				return pluginapi.Result{}, err
			}
			data["capabilities"] = capabilities
			if id, _ := i.Request.Input["item_id"].(string); id != "" {
				item, e := pluginwasm.EmbyGetItem(instance, id, "")
				if e != nil {
					return pluginapi.Result{}, e
				}
				data["item"] = item
			}
		}
		return pluginapi.Result{Message: "Emby 宿主合同查询完成", Data: data}, nil
	}})
}
