package pluginapi

import (
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

// Page contributes a namespaced route. Entry is optional: without it the host
// renders the existing declarative workspace as a full page. Frontends live
// exclusively under ui/ and are rendered in an opaque-origin sandbox.
type Page struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
	Entry       string `json:"entry,omitempty"`
	Navigation  bool   `json:"navigation,omitempty"`
}

func (d Definition) validatePages() error {
	if len(d.Pages) > 8 {
		return fmt.Errorf("插件独立页面最多 8 个")
	}
	seen := map[string]bool{}
	for _, p := range d.Pages {
		if !identifier.MatchString(p.ID) || seen[p.ID] || strings.TrimSpace(p.Label) == "" || utf8.RuneCountInString(p.Label) > 32 || len(p.Description) > 1024 || strings.ContainsAny(p.Label+p.Description, "\x00\r\n") {
			return fmt.Errorf("插件页面定义无效：%s", p.ID)
		}
		seen[p.ID] = true
		if p.Icon != "" && !identifier.MatchString(p.Icon) {
			return fmt.Errorf("插件页面图标无效：%s", p.ID)
		}
		if p.Entry != "" && (d.Runtime != "wasm" || !ValidPageAsset(p.Entry) || path.Ext(p.Entry) != ".html") {
			return fmt.Errorf("插件页面入口无效：%s", p.ID)
		}
	}
	return nil
}

// ValidPageAsset is shared by the installer, packer and static server. No
// manifests, WASM modules, source maps or private data are served as UI files.
func ValidPageAsset(name string) bool {
	if !strings.HasPrefix(name, "ui/") || len(name) > 240 || path.Clean(name) != name || strings.ContainsAny(name, "\\:%?#\x00\r\n") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || strings.HasPrefix(part, ".") {
			return false
		}
	}
	switch path.Ext(name) {
	case ".html", ".js", ".css", ".json", ".svg", ".png", ".jpg", ".jpeg", ".webp", ".woff2":
		return true
	}
	return false
}
