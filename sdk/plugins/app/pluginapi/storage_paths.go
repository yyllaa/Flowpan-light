package pluginapi

// Storage paths are private metadata. The host supplies plugin and account;
// namespace separates service/user/root identities across reconfiguration.
type StoragePathsRequest struct {
	Operation string   `json:"operation"`
	Namespace string   `json:"namespace"`
	Paths     []string `json:"paths,omitempty"`
	ID        string   `json:"id,omitempty"`
	Path      string   `json:"path,omitempty"`
}

type StoragePathRef struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}
