package pluginapi

// File jobs transfer bytes in the host. Plugins exchange object and task IDs,
// never cloud credentials, signed URLs or arbitrary local upload paths.
type FileStage struct {
	Key     string `json:"key"`
	Account string `json:"account"`
	ID      string `json:"id"`
}

type FilePublish struct {
	Key          string `json:"key"`
	Account      string `json:"account"`
	ParentID     string `json:"parent_id"`
	Name         string `json:"name"`
	MediaTask    string `json:"media_task,omitempty"`
	Artifact     string `json:"artifact,omitempty"`
	FileTask     string `json:"file_task,omitempty"`
	Conflict     string `json:"conflict"` // skip, keep_both, replace
	OriginalTask string `json:"original_task,omitempty"`
}

type FileObject struct {
	Account  string `json:"account"`
	SourceID string `json:"source_id"`
	ID       string `json:"id"`
	ParentID string `json:"parent_id"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
}

type FileTask struct {
	ID         string       `json:"id"`
	PluginID   string       `json:"plugin_id"`
	Operation  string       `json:"operation"`
	Status     string       `json:"status"`
	Phase      string       `json:"phase"`
	Progress   float64      `json:"progress"`
	Size       int64        `json:"size"`
	SHA256     string       `json:"sha256,omitempty"`
	Message    string       `json:"message,omitempty"`
	Result     *FileObject  `json:"result,omitempty"`
	Backup     *FileObject  `json:"backup,omitempty"`
	Stage      *FileStage   `json:"stage,omitempty"`
	Publish    *FilePublish `json:"publish,omitempty"`
	Restores   string       `json:"restores,omitempty"`
	CreatedAt  string       `json:"created_at"`
	FinishedAt string       `json:"finished_at,omitempty"`
}

type FileCapabilities struct {
	Account        string          `json:"account"`
	Stage          bool            `json:"stage"`
	Publish        bool            `json:"publish"`
	Conflicts      []string        `json:"conflicts"`
	AtomicReplace  bool            `json:"atomic_replace"`
	MaxFileBytes   int64           `json:"max_file_bytes"`
	CacheBytes     int64           `json:"cache_bytes"`
	MaxActiveTasks int             `json:"max_active_tasks"`
	Operations     map[string]bool `json:"operations"`
}
