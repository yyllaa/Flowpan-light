package pluginwasm

import "flowpan/app/pluginapi"

// StorageResult returns a native provider response without creating a plugin
// job. Use in Handlers.Storage; file bytes stay in the host transfer layer.
func StorageResult(value any) pluginapi.Result {
	return pluginapi.Result{Data: map[string]any{"storage": value}}
}
