package pluginapi

const (
	MaxHostRequest  = 64 << 10
	MaxHostResponse = 1 << 20
	MaxHostCalls    = 128
)

// HostError keeps the original human-readable error while exposing a stable code.
type HostError struct {
	Code    string `json:"code"`
	Method  string `json:"method"`
	Message string `json:"message"`
}

func (e *HostError) Error() string { return e.Message }

type HostLimits struct {
	RequestBytes      int `json:"request_bytes"`
	ResponseBytes     int `json:"response_bytes"`
	HostCalls         int `json:"host_calls"`
	MemoryMiB         int `json:"memory_mib"`
	MinTimeoutSeconds int `json:"min_timeout_seconds"`
	MaxTimeoutSeconds int `json:"max_timeout_seconds"`
}

func RuntimeLimits() HostLimits {
	return HostLimits{MaxHostRequest, MaxHostResponse, MaxHostCalls, 64, 60, 600}
}

type ToolAvailability struct {
	Tool
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type HostCapabilities struct {
	APIVersion        string             `json:"api_version"`
	DefinitionVersion int                `json:"definition_version"`
	Limits            HostLimits         `json:"limits"`
	Tools             []ToolAvailability `json:"tools"`
}
