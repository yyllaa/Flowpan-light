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
	RequestBytes               int   `json:"request_bytes"`
	ResponseBytes              int   `json:"response_bytes"`
	HostCalls                  int   `json:"host_calls"`
	MemoryMiB                  int   `json:"memory_mib"`
	MinTimeoutSeconds          int   `json:"min_timeout_seconds"`
	MaxTimeoutSeconds          int   `json:"max_timeout_seconds"`
	ArtifactBytes              int64 `json:"artifact_bytes"`
	ArtifactQuotaBytes         int64 `json:"artifact_quota_bytes"`
	ArtifactWriteBytes         int   `json:"artifact_write_bytes"`
	ServiceUploadBytes         int64 `json:"service_upload_bytes"`
	FileBytes                  int64 `json:"file_bytes"`
	ShareMaxSeconds            int   `json:"share_max_seconds"`
	InteractionAttachmentBytes int64 `json:"interaction_attachment_bytes"`
	InteractionSessionSeconds  int   `json:"interaction_session_seconds"`
	InteractionReplyBytes      int   `json:"interaction_reply_bytes"`
	InteractionButtons         int   `json:"interaction_buttons"`
}

func RuntimeLimits() HostLimits {
	return HostLimits{RequestBytes: MaxHostRequest, ResponseBytes: MaxHostResponse, HostCalls: MaxHostCalls, MemoryMiB: 64, MinTimeoutSeconds: 60, MaxTimeoutSeconds: 600, ArtifactBytes: 64 << 20, ArtifactQuotaBytes: 256 << 20, ArtifactWriteBytes: 32 << 10, ServiceUploadBytes: 64 << 20, FileBytes: 20 << 30, ShareMaxSeconds: 86400, InteractionAttachmentBytes: 20 << 20, InteractionSessionSeconds: 1800, InteractionReplyBytes: 16 << 10, InteractionButtons: 8}
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
