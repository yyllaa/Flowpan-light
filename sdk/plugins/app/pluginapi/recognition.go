package pluginapi

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Recognition providers supply identities. The host retains routing, naming,
// authorization and all file operations. Provider IDs are local to a plugin.
type RecognitionDefinition struct {
	Providers  []RecognitionProvider `json:"providers"`
	Categories []RecognitionCategory `json:"categories,omitempty"`
}
type RecognitionCategory struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type RecognitionCategoryInfo struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Plugin  string `json:"plugin"`
	Enabled bool   `json:"enabled"`
}
type RecognitionProvider struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Extensions []string `json:"extensions,omitempty"`
	Patterns   []string `json:"patterns,omitempty"`
	Keywords   []string `json:"keywords,omitempty"`
	Category   string   `json:"category,omitempty"`
}
type RecognitionProviderInfo struct {
	RecognitionProvider
	Key          string `json:"key"`
	Plugin       string `json:"plugin"`
	Version      string `json:"version"`
	Enabled      bool   `json:"enabled"`
	CategoryKey  string `json:"category_key,omitempty"`
	CategoryName string `json:"category_name,omitempty"`
}
type RecognitionInput struct {
	Query        string `json:"query,omitempty"`
	Account      string `json:"account"`
	ID           string `json:"id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	RelativePath string `json:"relative_path,omitempty"`
	Size         int64  `json:"size,omitempty"`
}
type RecognitionCall struct {
	Provider string           `json:"provider"`
	File     RecognitionInput `json:"file"`
}
type RecognitionMetadata struct {
	Provider    string            `json:"provider,omitempty"`
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Type        string            `json:"type"`
	Year        string            `json:"year,omitempty"`
	Season      *int              `json:"season,omitempty"`
	Episode     *int              `json:"episode,omitempty"`
	IDs         map[string]string `json:"ids,omitempty"`
	ReleaseDate string            `json:"release_date,omitempty"`
	Studio      string            `json:"studio,omitempty"`
	People      []string          `json:"people,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}
type RecognitionResult struct {
	File        *RecognitionInput        `json:"file,omitempty"`
	Status      string                   `json:"status"`
	Provider    string                   `json:"provider,omitempty"`
	Package     string                   `json:"package,omitempty"`
	Metadata    *RecognitionMetadata     `json:"metadata,omitempty"`
	Candidates  []RecognitionMetadata    `json:"candidates,omitempty"`
	Reason      string                   `json:"reason,omitempty"`
	Naming      *RecognitionNaming       `json:"naming,omitempty"`
	Destination *RecognitionDestination  `json:"destination,omitempty"`
	Names       *OrganizeNames           `json:"names,omitempty"`
	Category    *RecognitionCategoryInfo `json:"category,omitempty"`
}

// Templates are individual path components. Only the host renders them.
type RecognitionNaming struct {
	Directories []string `json:"directories,omitempty"`
	File        string   `json:"file,omitempty"`
}

// Destination is selected by the task, never by the provider. ID is resolved
// against the input account; Path is a display label, not an authority.
type RecognitionDestination struct {
	ID   string `json:"id"`
	Path string `json:"path,omitempty"`
}

func (d *RecognitionDestination) Validate() error {
	if d != nil && (strings.TrimSpace(d.ID) == "" || len(d.ID) > 4096 || len(d.Path) > 4096 || strings.ContainsAny(d.ID+d.Path, "\x00\r\n")) {
		return fmt.Errorf("请选择有效的识别整理目标目录")
	}
	return nil
}

type RecognitionRoute struct {
	Mode        string                  `json:"mode"`
	Providers   []string                `json:"providers,omitempty"`
	Rules       []RecognitionRule       `json:"rules,omitempty"`
	Naming      *RecognitionNaming      `json:"naming,omitempty"`
	Destination *RecognitionDestination `json:"destination,omitempty"`
}
type RecognitionRule struct {
	Prefix      string                  `json:"prefix"`
	Mode        string                  `json:"mode"`
	Providers   []string                `json:"providers,omitempty"`
	Naming      *RecognitionNaming      `json:"naming,omitempty"`
	Destination *RecognitionDestination `json:"destination,omitempty"`
}
type RecognitionIdentify struct {
	Account string            `json:"account"`
	ID      string            `json:"id"`
	Route   *RecognitionRoute `json:"route"`
}

func (d RecognitionDefinition) Validate() error {
	if len(d.Providers) < 1 || len(d.Providers) > 16 {
		return fmt.Errorf("识别来源须为 1–16 个")
	}

	categories := map[string]bool{}
	if len(d.Categories) > 16 {
		return fmt.Errorf("插件分类最多 16 个")
	}
	for _, c := range d.Categories {
		if !recognitionID.MatchString(c.ID) || categories[c.ID] || strings.TrimSpace(c.Name) == "" || len(c.Name) > 120 || strings.ContainsAny(c.Name, "/\\\x00\r\n") {
			return fmt.Errorf("插件分类声明无效")
		}
		categories[c.ID] = true
	}
	seen := map[string]bool{}
	for _, p := range d.Providers {
		if !recognitionID.MatchString(p.ID) || seen[p.ID] || strings.TrimSpace(p.Name) == "" || len(p.Name) > 120 || len(p.Extensions) > 32 || len(p.Patterns) > 16 {
			return fmt.Errorf("识别来源声明无效: %s", p.ID)
		}

		if len(p.Keywords) > 64 || p.Category != "" && !categories[p.Category] {
			return fmt.Errorf("插件识别关键词或分类无效")
		}
		for _, word := range p.Keywords {
			if len(strings.TrimSpace(word)) < 2 || len(word) > 120 || strings.ContainsAny(word, "\x00\r\n") {
				return fmt.Errorf("插件识别关键词无效")
			}
		}
		seen[p.ID] = true
		for _, ext := range p.Extensions {
			if !regexp.MustCompile(`^\.[a-z0-9]{1,12}$`).MatchString(ext) {
				return fmt.Errorf("识别来源扩展名无效")
			}
		}
		for _, pattern := range p.Patterns {
			if len(pattern) == 0 || len(pattern) > 1024 {
				return fmt.Errorf("识别来源匹配规则无效")
			}
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("识别来源匹配规则无效: %w", err)
			}
		}
	}
	return nil
}

var recognitionID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

func SplitRecognitionProvider(key string) (string, string, error) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 || !recognitionID.MatchString(parts[0]) || !recognitionID.MatchString(parts[1]) {
		return "", "", fmt.Errorf("识别来源标识无效")
	}
	return parts[0], parts[1], nil
}
func (p RecognitionProvider) Matches(name string) bool {
	if len(p.Extensions) > 0 {
		found := false
		for _, ext := range p.Extensions {
			found = found || strings.EqualFold(path.Ext(name), ext)
		}
		if !found {
			return false
		}
	}
	for _, word := range p.Keywords {
		if strings.Contains(strings.ToLower(name), strings.ToLower(strings.TrimSpace(word))) {
			return true
		}
	}
	// Mixed routing requires an explicit, cheap filename claim. A provider with
	// no patterns remains available for bound tasks and manual invocation.
	for _, pattern := range p.Patterns {
		if re, err := regexp.Compile(pattern); err == nil && re.MatchString(name) {
			return true
		}
	}
	return false
}
func (r *RecognitionRoute) Validate() error {
	if r == nil {
		return nil
	}
	if err := r.Destination.Validate(); err != nil {
		return err
	}
	if err := validateRecognitionSelection(r.Mode, r.Providers, r.Naming); err != nil {
		return err
	}
	if len(r.Rules) > 16 {
		return fmt.Errorf("识别目录规则最多 16 条")
	}
	seen := map[string]bool{}
	for _, rule := range r.Rules {
		p := rule.Prefix
		if p == "" || path.Clean(p) != p || p == "." || p == ".." || strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\:\x00\r\n") || len(p) > 1024 || seen[p] {
			return fmt.Errorf("识别目录须为不重复的相对目录")
		}
		seen[p] = true
		if err := rule.Destination.Validate(); err != nil {
			return err
		}
		if err := validateRecognitionSelection(rule.Mode, rule.Providers, rule.Naming); err != nil {
			return err
		}
	}
	return nil
}
func validateRecognitionSelection(mode string, providers []string, naming *RecognitionNaming) error {
	if mode != "" && mode != "native" && mode != "provider" && mode != "auto" {
		return fmt.Errorf("识别路由模式无效")
	}
	if len(providers) > 8 || ((mode == "provider" || mode == "auto") && len(providers) == 0) {
		return fmt.Errorf("请选择 1–8 个识别来源")
	}
	seen := map[string]bool{}
	for _, p := range providers {
		if _, _, err := SplitRecognitionProvider(p); err != nil {
			return err
		}
		if seen[p] {
			return fmt.Errorf("识别来源重复")
		}
		seen[p] = true
	}
	if naming != nil {
		if len(naming.Directories) > 8 {
			return fmt.Errorf("识别命名目录最多 8 层")
		}
		for _, part := range append(append([]string{}, naming.Directories...), naming.File) {
			if part == "" {
				continue
			}
			if len(part) > 512 || strings.ContainsAny(part, "/\\\x00\r\n") || part == "." || part == ".." {
				return fmt.Errorf("识别命名须为单个路径组件")
			}
		}
	}
	return nil
}

// Select applies the longest literal directory prefix, never a substring of a
// filename. Its input is computed from the authorized task root by the host.
func (r *RecognitionRoute) Select(relativePath string) RecognitionRoute {
	if r == nil {
		return RecognitionRoute{Mode: "native"}
	}
	selected := *r
	selected.Rules = nil
	best := -1
	dir := path.Dir(strings.ReplaceAll(relativePath, "\\", "/"))
	for _, rule := range r.Rules {
		if (dir == rule.Prefix || strings.HasPrefix(dir, rule.Prefix+"/")) && len(rule.Prefix) > best {
			best = len(rule.Prefix)
			selected = RecognitionRoute{Mode: rule.Mode, Providers: rule.Providers, Naming: rule.Naming, Destination: rule.Destination}
			if selected.Destination == nil {
				selected.Destination = r.Destination
			}
			if selected.Naming == nil {
				selected.Naming = r.Naming
			}
		}
	}
	return selected
}
func (m RecognitionMetadata) Validate() error {
	if strings.TrimSpace(m.ID) == "" || len(m.ID) > 256 || strings.TrimSpace(m.Title) == "" || len(m.Title) > 240 || (m.Type != "movie" && m.Type != "tv") || len(m.Year) > 4 || len(m.ReleaseDate) > 32 || len(m.Studio) > 240 || len(m.IDs) > 16 || len(m.People) > 100 || len(m.Tags) > 100 {
		return fmt.Errorf("识别结果媒体身份无效")
	}
	if m.Season != nil && (*m.Season < 0 || *m.Season > 999) || m.Episode != nil && (*m.Episode < 0 || *m.Episode > 9999) {
		return fmt.Errorf("识别结果季集无效")
	}
	for k, v := range m.IDs {
		if !recognitionID.MatchString(k) || len(v) > 256 {
			return fmt.Errorf("识别结果外部标识无效")
		}
	}
	for _, v := range append(append([]string{}, m.People...), m.Tags...) {
		if len(v) > 240 {
			return fmt.Errorf("识别结果字段过长")
		}
	}
	return nil
}
func (r RecognitionResult) Validate() error {
	if len(r.Reason) > 2048 || len(r.Candidates) > 16 {
		return fmt.Errorf("识别结果过大")
	}
	switch r.Status {
	case "matched":
		if r.Metadata == nil {
			return fmt.Errorf("识别成功须返回媒体身份")
		}
		return r.Metadata.Validate()
	case "no_match", "not_found", "ambiguous", "error":
		for _, c := range r.Candidates {
			if err := c.Validate(); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("识别结果状态无效")
	}
}

func recognitionTools() []Tool {
	return []Tool{
		tool("recognition.providers", "media.read", "识别与 AI", "列出已安装插件声明的识别来源及可用状态", "", false, false),
		tool("recognition.identify", "media.read", "识别与 AI", "按任务路由调用新识别来源，返回独立媒体身份和待确认原因，不移动文件", "account:string id:string route?:object", false, false),
	}
}
