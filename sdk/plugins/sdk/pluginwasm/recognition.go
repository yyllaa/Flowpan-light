package pluginwasm

import (
	"flowpan/app/pluginapi"
	"fmt"
)

func RecognitionContext(v Invocation) (pluginapi.RecognitionCall, error) {
	if v.Mode != "recognition" || v.Request.Recognition == nil {
		return pluginapi.RecognitionCall{}, fmt.Errorf("缺少宿主识别上下文")
	}
	return *v.Request.Recognition, nil
}
func RecognitionReply(result pluginapi.RecognitionResult) (pluginapi.Result, error) {
	if err := result.Validate(); err != nil {
		return pluginapi.Result{}, err
	}
	return pluginapi.Result{Data: map[string]any{"recognition": result}}, nil
}
func RecognitionProviders() ([]pluginapi.RecognitionProviderInfo, error) {
	var result []pluginapi.RecognitionProviderInfo
	err := Call("recognition.providers", map[string]any{}, &result)
	return result, err
}
func IdentifyRecognition(input pluginapi.RecognitionIdentify) (pluginapi.RecognitionResult, error) {
	var result pluginapi.RecognitionResult
	err := Call("recognition.identify", input, &result)
	return result, err
}
