package pluginapi

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// Rules are evaluated locally by the host. Matching a pasted link never runs
// WASM or queries the cloud. A plugin cannot claim built-in or arbitrary URLs.
type StorageShareRule struct {
	Host        string   `json:"host"`
	PathPattern string   `json:"path_pattern"`
	QueryKeys   []string `json:"query_keys,omitempty"`
}

func (r StorageShareRule) Validate() error {
	h := strings.ToLower(r.Host)
	if h != r.Host || len(h) > 253 || !strings.Contains(h, ".") || strings.ContainsAny(h, "/\\:*?#@ \t\r\n") || net.ParseIP(h) != nil || strings.HasSuffix(h, ".localhost") {
		return fmt.Errorf("分享链接域名无效")
	}
	for _, reserved := range []string{"115.com", "115cdn.com", "anxia.com", "139.com"} {
		if h == reserved || strings.HasSuffix(h, "."+reserved) {
			return fmt.Errorf("分享链接域名已由项目管理")
		}
	}
	if len(r.PathPattern) > 256 || !strings.HasPrefix(r.PathPattern, "^/") {
		return fmt.Errorf("分享链接路径规则无效")
	}
	re, err := regexp.Compile(r.PathPattern)
	if err != nil || re.MatchString("/") || len(r.QueryKeys) > 12 {
		return fmt.Errorf("分享链接路径或查询规则无效")
	}
	seen := map[string]bool{}
	for _, key := range r.QueryKeys {
		if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`).MatchString(key) || seen[key] {
			return fmt.Errorf("分享链接查询参数无效")
		}
		seen[key] = true
	}
	return nil
}

func (r StorageShareRule) Match(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimRight(raw, "。；;）)]】}"))
	if err != nil || u.Scheme != "https" || u.User != nil || !strings.EqualFold(u.Hostname(), r.Host) || (u.Port() != "" && u.Port() != "443") {
		return "", false
	}
	re, err := regexp.Compile(r.PathPattern)
	if err != nil {
		return "", false
	}
	m := re.FindStringIndex(u.Path)
	if m == nil || m[0] != 0 || m[1] <= 1 {
		return "", false
	}
	q := url.Values{}
	for _, key := range r.QueryKeys {
		if v := u.Query().Get(key); v != "" && len(v) <= 2048 {
			q.Set(key, v)
		}
	}
	u.Scheme, u.Host, u.Path, u.RawPath, u.RawQuery, u.Fragment = "https", r.Host, u.Path[:m[1]], "", q.Encode(), ""
	return u.String(), true
}

type StorageShareRequest struct {
	URL      string            `json:"url"`
	Password string            `json:"password,omitempty"`
	Info     *StorageShareInfo `json:"info,omitempty"`
	TaskID   string            `json:"task_id,omitempty"`
}
type StorageShareEntry struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
}
type StorageShareInfo struct {
	ID      string              `json:"id"`
	Name    string              `json:"name"`
	Entries []StorageShareEntry `json:"entries"`
	Context map[string]string   `json:"context,omitempty"`
}
type StorageShareReceipt struct {
	Done    bool   `json:"done"`
	TaskID  string `json:"task_id,omitempty"`
	Count   int    `json:"count"`
	Message string `json:"message,omitempty"`
}
