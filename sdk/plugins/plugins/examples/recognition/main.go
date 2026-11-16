// This is a provider contract scaffold, not a metadata website implementation.
package main

import (
	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
)

func main() {
	pluginwasm.Serve(pluginwasm.Handlers{Recognition: func(i pluginwasm.Invocation) (pluginapi.Result, error) {
		call, err := pluginwasm.RecognitionContext(i)
		if err != nil {
			return pluginapi.Result{}, err
		}
		// Implement a provider-specific lookup using call.File and declared host
		// tools. A filename pattern is a claim, not proof of a media identity.
		_ = call
		return pluginwasm.RecognitionReply(pluginapi.RecognitionResult{Status: "no_match", Reason: "请实现来源查询并确认媒体身份"})
	}})
}
