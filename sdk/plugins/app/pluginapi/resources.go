package pluginapi

// BrowserAccess declares the network origins and secret fields used by a plugin.
// Executables, profiles and browser maintenance belong to the project.
type BrowserAccess struct {
	Hosts       []string `json:"hosts"`
	Credentials []string `json:"credentials,omitempty"`
}

// ServiceConnection binds an optional external service to declared configuration
// fields. Credentials are inserted by the host, never exposed to the WASM guest.
type ServiceConnection struct {
	URLField    string   `json:"url_field"`
	TokenField  string   `json:"token_field"`
	Credentials []string `json:"credentials,omitempty"`
}

// ResourceProvider adds the same resources workspace to any external plugin.
type ResourceProvider struct {
	SearchAction string `json:"search_action"`
	UnlockAction string `json:"unlock_action"`
	FilterAction string `json:"filter_action,omitempty"`
}

type Resource struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Size    string   `json:"size,omitempty"`
	Points  int      `json:"points"`
	Owned   bool     `json:"owned"`
	PanType string   `json:"pan_type"`
	Files   []string `json:"files,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

type ResourceLinks struct {
	Links    []string `json:"links"`
	Password string   `json:"password,omitempty"`
	Balance  *int     `json:"balance,omitempty"`
	Already  bool     `json:"already,omitempty"`
}
