package pluginwasm

import "flowpan/app/pluginapi"

// CapabilityStatuses inspects only dependencies declared by this plugin.
func CapabilityStatuses() ([]pluginapi.CapabilityBindingStatus, error) {
	var out []pluginapi.CapabilityBindingStatus
	err := Call("capabilities.list", nil, &out)
	return out, err
}

// CallCapability decodes the provider's validated data into target. The host
// selects the saved provider; guest input cannot select another plugin/action.
func CallCapability(use string, input map[string]any, target any) error {
	return Call("capabilities.call", pluginapi.CapabilityCall{Use: use, Input: input}, target)
}

type CapabilityTask struct {
	ID       string             `json:"id"`
	Status   string             `json:"status"`
	Provider string             `json:"provider"`
	Progress pluginapi.Progress `json:"progress"`
	Data     map[string]any     `json:"data,omitempty"`
	Message  string             `json:"message,omitempty"`
}

func SubmitCapability(use, key string, input map[string]any) (CapabilityTask, error) {
	var out CapabilityTask
	err := Call("capabilities.submit", pluginapi.CapabilityCall{Use: use, Key: key, Input: input}, &out)
	return out, err
}

func GetCapabilityTask(id string) (CapabilityTask, error) {
	var out CapabilityTask
	err := Call("capabilities.task", pluginapi.CapabilityCall{ID: id}, &out)
	return out, err
}

func CancelCapabilityTask(id string) error {
	return Call("capabilities.cancel", pluginapi.CapabilityCall{ID: id}, nil)
}
