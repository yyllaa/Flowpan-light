package pluginapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// StorageDefinition contributes a provider to the application's native account
// and file APIs. Fields belong to each account, independently of plugin config.
type StorageDefinition struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Icon         string             `json:"icon,omitempty"`
	LoginMethods []string           `json:"login_methods"`
	Fields       []Field            `json:"fields,omitempty"`
	Capabilities []string           `json:"capabilities"`
	TaskFields   map[string][]Field `json:"task_fields,omitempty"`
	ShareLinks   []StorageShareRule `json:"share_links,omitempty"`
}

func (d StorageDefinition) Has(capability string) bool {
	for _, c := range d.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

func (d StorageDefinition) Validate() error {
	if len(d.ID) == 0 || len(d.ID) > 64 || d.ID[0] < 'a' || d.ID[0] > 'z' || d.ID == "p115" || d.ID == "p139" || strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("网盘插件标识或名称无效")
	}
	for _, c := range d.ID {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return fmt.Errorf("网盘插件标识无效")
		}
	}
	seen := map[string]bool{}
	for _, c := range d.Capabilities {
		switch c {
		case "list", "stat", "read", "link", "upload", "mkdir", "rename", "delete", "copy", "move", "refresh", "share":
		default:
			return fmt.Errorf("网盘能力未注册：%s", c)
		}
		if seen[c] {
			return fmt.Errorf("网盘能力重复：%s", c)
		}
		seen[c] = true
	}
	if !seen["list"] || !seen["stat"] || len(d.LoginMethods) == 0 {
		return fmt.Errorf("网盘插件缺少浏览能力或登录方式")
	}
	if len(d.ShareLinks) > 16 || (len(d.ShareLinks) > 0) != seen["share"] {
		return fmt.Errorf("分享能力与链接声明不一致")
	}
	for _, rule := range d.ShareLinks {
		if err := rule.Validate(); err != nil {
			return err
		}
	}
	seen = map[string]bool{}
	for _, method := range d.LoginMethods {
		if !identifier.MatchString(method) || seen[method] {
			return fmt.Errorf("网盘登录方式无效")
		}
		seen[method] = true
	}
	if err := ValidateStorageFields(d.Fields, false); err != nil {
		return err
	}
	for scope, fields := range d.TaskFields {
		switch scope {
		case "strm", "upload", "organize", "audiobook", "fuse":
		default:
			return fmt.Errorf("网盘任务类型无效：%s", scope)
		}
		if err := ValidateStorageFields(fields, true); err != nil {
			return err
		}
	}
	return nil
}

// StorageCall is host-created. Public plugin actions cannot populate this call.
// Credentials and settings are delivered only to the owning provider module.
type StorageCall struct {
	Operation    string          `json:"operation"`
	Account      string          `json:"account"`
	SourceID     string          `json:"source_id,omitempty"`
	Settings     map[string]any  `json:"settings,omitempty"`
	Credentials  json.RawMessage `json:"credentials,omitempty"`
	Session      json.RawMessage `json:"session,omitempty"`
	Context      string          `json:"context,omitempty"`
	TaskSettings map[string]any  `json:"task_settings,omitempty"`
	Params       StorageParams   `json:"params"`
}
type StorageParams struct {
	ID         string               `json:"id,omitempty"`
	ParentID   string               `json:"parent_id,omitempty"`
	IDs        []string             `json:"ids,omitempty"`
	Name       string               `json:"name,omitempty"`
	Cursor     string               `json:"cursor,omitempty"`
	Offset     int                  `json:"offset,omitempty"`
	Limit      int                  `json:"limit,omitempty"`
	Order      string               `json:"order,omitempty"`
	Asc        bool                 `json:"asc,omitempty"`
	Keyword    string               `json:"keyword,omitempty"`
	ReadOffset int64                `json:"read_offset,omitempty"`
	ReadLength int64                `json:"read_length,omitempty"`
	UserAgent  string               `json:"user_agent,omitempty"`
	Method     string               `json:"method,omitempty"`
	Refresh    bool                 `json:"refresh,omitempty"`
	Upload     *StorageUpload       `json:"upload,omitempty"`
	Field      string               `json:"field,omitempty"`
	Share      *StorageShareRequest `json:"share,omitempty"`
}
type StorageHash struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}
type StorageEntry struct {
	ID       string        `json:"id"`
	ParentID string        `json:"parent_id"`
	Name     string        `json:"name"`
	Path     string        `json:"path,omitempty"`
	IsDir    bool          `json:"is_dir"`
	Size     int64         `json:"size"`
	Modified int64         `json:"modified,omitempty"`
	Version  string        `json:"version,omitempty"`
	Hashes   []StorageHash `json:"hashes,omitempty"`
}
type StoragePage struct {
	Items      []StorageEntry `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
	HasMore    bool           `json:"has_more"`
	Total      int            `json:"total"`
	Offset     int            `json:"offset"`
	Limit      int            `json:"limit"`
}
type StorageLink struct {
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers,omitempty"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
}
type StorageAuth struct {
	State       string          `json:"state"`
	Identity    string          `json:"identity,omitempty"`
	RootID      string          `json:"root_id,omitempty"`
	Name        string          `json:"name,omitempty"`
	Credentials json.RawMessage `json:"credentials,omitempty"`
	ExpiresAt   time.Time       `json:"expires_at,omitempty"`
	Session     json.RawMessage `json:"session,omitempty"`
	QRCode      string          `json:"qrcode,omitempty"`
	Stage       string          `json:"stage,omitempty"`
	Message     string          `json:"message,omitempty"`
	SizeUsed    string          `json:"size_used,omitempty"`
	SizeTotal   string          `json:"size_total,omitempty"`
	Values      map[string]any  `json:"values,omitempty"`
}
type StorageUpload struct {
	Size       int64                `json:"size"`
	PartSize   int64                `json:"part_size,omitempty"`
	Hashes     []StorageHash        `json:"hashes,omitempty"`
	Checkpoint json.RawMessage      `json:"checkpoint,omitempty"`
	Response   *StorageHTTPResponse `json:"response,omitempty"`
	Resuming   bool                 `json:"resuming,omitempty"`
}
type StorageHTTPRequest struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers,omitempty"`
	Offset    int64             `json:"offset"`
	Length    int64             `json:"length"`
	Multipart *StorageMultipart `json:"multipart,omitempty"`
}

// Multipart wraps the file slice in a bounded form without copying file bytes
// through the guest. The host generates the boundary and Content-Length.
type StorageMultipart struct {
	FileField string             `json:"file_field"`
	FileName  string             `json:"file_name"`
	Fields    []StorageFormField `json:"fields,omitempty"`
}
type StorageFormField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// StorageCredentials is an optional storage_credentials result. Rotation keeps
// the account identity, settings and authorization revision unchanged.
type StorageCredentials struct {
	Credentials json.RawMessage `json:"credentials"`
	ExpiresAt   time.Time       `json:"expires_at,omitempty"`
}
type StorageHTTPResponse struct {
	URL     string            `json:"url,omitempty"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// An upload step either returns a completed entry or one bounded file slice.
// The host persists Checkpoint before sending bytes, and returns the receipt to
// the next invocation. File bytes never pass through the JSON/WASM ABI.
type StorageUploadStep struct {
	Entry          *StorageEntry       `json:"entry,omitempty"`
	Reused         bool                `json:"reused,omitempty"`
	Request        *StorageHTTPRequest `json:"request,omitempty"`
	Checkpoint     json.RawMessage     `json:"checkpoint,omitempty"`
	HashAlgorithm  string              `json:"hash_algorithm,omitempty"`
	SaveCheckpoint bool                `json:"save_checkpoint,omitempty"`
}
