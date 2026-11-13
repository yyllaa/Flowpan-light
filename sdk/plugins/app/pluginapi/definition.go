// Package pluginapi defines the host-owned, declarative plugin contract.
// It has no dependency on Flowpan services, HTTP handlers or a UI framework.
package pluginapi

import (
	"fmt"
	"regexp"
	"strings"
)

const SchemaVersion = 2
const APIVersion = "1"

type Definition struct {
	SchemaVersion int                `json:"schema_version"`
	APIVersion    string             `json:"api_version"`
	ID            string             `json:"id"`
	Version       string             `json:"version"`
	Name          string             `json:"name"`
	Description   string             `json:"description"`
	Category      string             `json:"category"`
	Icon          string             `json:"icon"`
	Runtime       string             `json:"runtime"`
	ConfigVersion int                `json:"config_version"`
	ToggleField   string             `json:"toggle_field,omitempty"`
	Permissions   []string           `json:"permissions,omitempty"`
	Fields        []Field            `json:"fields,omitempty"`
	Actions       []Action           `json:"actions"`
	ResultFields  []ResultField      `json:"result_fields,omitempty"`
	LegacyUI      bool               `json:"legacy_ui,omitempty"`
	HTTPHosts     []string           `json:"http_hosts,omitempty"`
	Network       *NetworkPolicy     `json:"network,omitempty"`
	Schedules     []Schedule         `json:"schedules,omitempty"`
	Subscriptions []Subscription     `json:"subscriptions,omitempty"`
	Contributions []Contribution     `json:"contributions,omitempty"`
	Service       *ServiceConnection `json:"service,omitempty"`
	Resources     *ResourceProvider  `json:"resources,omitempty"`
	SDKs          []SDKAccess        `json:"sdks,omitempty"`
	Browser       *BrowserAccess     `json:"browser,omitempty"`
	Storage       *StorageDefinition `json:"storage,omitempty"`
}

type Field struct {
	Key           string     `json:"key"`
	Label         string     `json:"label"`
	Type          string     `json:"type"`
	Required      bool       `json:"required,omitempty"`
	RuntimeAccess bool       `json:"runtime_access,omitempty"`
	ReadOnly      bool       `json:"read_only,omitempty"`
	Hint          string     `json:"hint,omitempty"`
	Options       []Option   `json:"options,omitempty"`
	VisibleWhen   *Condition `json:"visible_when,omitempty"`
	Default       any        `json:"default,omitempty"`
	Min           *float64   `json:"min,omitempty"`
	Max           *float64   `json:"max,omitempty"`
	Step          *float64   `json:"step,omitempty"`
	Group         string     `json:"group,omitempty"`
	OptionsSource string     `json:"options_source,omitempty"`
	DependsOn     []string   `json:"depends_on,omitempty"`
}

// Conditions refer to field IDs. The renderer never evaluates source code.
type Condition struct {
	Field  string `json:"field"`
	Equals any    `json:"equals"`
}

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// MediaInput binds the host's TMDB picker to declared action input fields.
type MediaInput struct {
	IDField   string `json:"id_field"`
	TypeField string `json:"type_field"`
}

type InputGroup struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Collapsible bool   `json:"collapsible,omitempty"`
}

type Action struct {
	ChoiceField     string           `json:"choice_field,omitempty"`
	Choices         []Option         `json:"choices,omitempty"`
	Description     string           `json:"description,omitempty"`
	MediaInput      *MediaInput      `json:"media_input,omitempty"`
	InputGroups     []InputGroup     `json:"input_groups,omitempty"`
	ResultFields    []ResultField    `json:"result_fields,omitempty"`
	Placement       string           `json:"placement,omitempty"`
	InputFields     []Field          `json:"input_fields,omitempty"`
	Idempotent      bool             `json:"idempotent,omitempty"`
	Cancellable     bool             `json:"cancellable,omitempty"`
	TimeoutSeconds  int              `json:"timeout_seconds,omitempty"`
	ID              string           `json:"id"`
	Label           string           `json:"label"`
	Kind            string           `json:"kind"`
	Icon            string           `json:"icon"`
	RequiresEnabled bool             `json:"requires_enabled,omitempty"`
	Danger          bool             `json:"danger,omitempty"`
	Confirmation    string           `json:"confirmation,omitempty"`
	RequiresResult  *ResultCondition `json:"requires_result,omitempty"`
}

type ResultCondition struct {
	Field       string  `json:"field"`
	GreaterThan float64 `json:"greater_than"`
}

type ResultField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type State struct {
	Enabled    bool   `json:"enabled"`
	Configured bool   `json:"configured"`
	Label      string `json:"label"`
	Detail     string `json:"detail,omitempty"`
}

type ResultItem struct {
	Label   string `json:"label"`
	Message string `json:"message"`
	Failed  bool   `json:"failed,omitempty"`
}

type ResultTable struct {
	Columns []ResultField       `json:"columns"`
	Rows    []map[string]string `json:"rows"`
}

type Result struct {
	Table   *ResultTable   `json:"table,omitempty"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
	Items   []ResultItem   `json:"items,omitempty"`
	Failed  bool           `json:"failed,omitempty"`
}

type Snapshot struct {
	Definition Definition          `json:"definition"`
	Config     map[string]any      `json:"config"`
	Secrets    map[string]bool     `json:"secrets,omitempty"`
	Options    map[string][]Option `json:"options,omitempty"`
	State      State               `json:"state"`
	Result     *Result             `json:"result,omitempty"`
	Running    bool                `json:"running"`
	Schedules  []ScheduleStatus    `json:"schedules,omitempty"`
}

var identifier = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

func (d Definition) Validate() error {
	if d.SchemaVersion != SchemaVersion || d.APIVersion != APIVersion {
		return fmt.Errorf("插件合同版本不兼容")
	}
	if !identifier.MatchString(d.ID) || d.Name == "" || d.Version == "" || d.ConfigVersion < 1 || (d.Runtime != "builtin" && d.Runtime != "wasm") {
		return fmt.Errorf("插件定义不完整")
	}
	if err := d.validateNetwork(); err != nil {
		return err
	}
	fields := map[string]Field{}
	for _, field := range d.Fields {
		if !identifier.MatchString(field.Key) || field.Label == "" {
			return fmt.Errorf("插件配置字段无效：%s", field.Key)
		}
		if _, exists := fields[field.Key]; exists {
			return fmt.Errorf("插件配置字段重复：%s", field.Key)
		}
		switch field.Type {
		case "boolean", "text", "cron", "account-list", "url", "path", "select", "secret", "number", "directory-list":
		default:
			return fmt.Errorf("插件配置字段类型未注册：%s", field.Type)
		}
		if field.RuntimeAccess && field.Type != "secret" {
			return fmt.Errorf("运行时凭据访问只适用于密钥字段：%s", field.Key)
		}
		seen := map[string]bool{}
		for _, option := range field.Options {
			if option.Value == "" || option.Label == "" || seen[option.Value] {
				return fmt.Errorf("插件字段选项无效：%s", field.Key)
			}
			seen[option.Value] = true
		}
		fields[field.Key] = field
	}
	if d.ToggleField != "" && fields[d.ToggleField].Type != "boolean" {
		return fmt.Errorf("插件开关字段无效")
	}
	for _, field := range d.Fields {
		if c := field.VisibleWhen; c != nil && (c.Field == field.Key || fields[c.Field].Type != "boolean") {
			return fmt.Errorf("插件显示条件无效：%s", field.Key)
		}
	}
	if err := d.validateSDKs(fields); err != nil {
		return err
	}
	actions := map[string]bool{}
	if len(d.Actions) == 0 && d.Storage == nil {
		return fmt.Errorf("插件缺少动作")
	}
	for _, action := range d.Actions {
		if !identifier.MatchString(action.ID) || action.Label == "" || actions[action.ID] {
			return fmt.Errorf("插件动作无效：%s", action.ID)
		}
		if action.Kind != "run" && action.Kind != "probe" {
			return fmt.Errorf("插件动作类型未注册")
		}
		if action.Danger && action.Confirmation == "" {
			return fmt.Errorf("插件动作缺少确认文案")
		}
		if err := validateActionInputs(action); err != nil {
			return err
		}
		actions[action.ID] = true
	}
	choiceFields := map[string]bool{}
	for _, action := range d.Actions {
		if len(action.Choices) == 0 {
			if action.ChoiceField != "" {
				return fmt.Errorf("插件动作选项字段缺少选项：%s", action.ID)
			}
			continue
		}
		if len(action.Choices) < 2 || len(action.Choices) > 8 || action.Placement != "" || action.Choices[0].Value != action.ID {
			return fmt.Errorf("插件动作选项无效：%s", action.ID)
		}
		if action.ChoiceField != "" {
			f := fields[action.ChoiceField]
			if f.Type != "select" || f.ReadOnly || f.VisibleWhen != nil || choiceFields[f.Key] || len(f.Options) != len(action.Choices) {
				return fmt.Errorf("插件动作选项配置无效：%s", action.ID)
			}
			values := map[string]bool{}
			for _, choice := range action.Choices {
				values[choice.Value] = true
			}
			for _, option := range f.Options {
				if !values[option.Value] {
					return fmt.Errorf("插件动作选项配置不匹配：%s", action.ID)
				}
			}
			if f.Default != nil && !values[fmt.Sprint(f.Default)] {
				return fmt.Errorf("插件动作默认选项无效：%s", action.ID)
			}
			choiceFields[f.Key] = true
		}
		seen := map[string]bool{}
		for _, choice := range action.Choices {
			if !actions[choice.Value] || choice.Label == "" || seen[choice.Value] {
				return fmt.Errorf("插件动作选项无效：%s", action.ID)
			}
			seen[choice.Value] = true
			for _, target := range d.Actions {
				if target.ID == choice.Value && (len(target.InputFields) > 0 || target.ID != action.ID && (target.Placement != "tools" || len(target.Choices) > 0)) {
					return fmt.Errorf("插件动作选项需要无输入的独立动作：%s", choice.Value)
				}
			}
		}
	}
	if err := d.validateSchedules(fields, actions); err != nil {
		return err
	}
	for _, sub := range d.Subscriptions {
		if !actions[sub.Action] || !ValidEventType(sub.Type) {
			return fmt.Errorf("插件事件订阅无效")
		}
	}
	for _, c := range d.Contributions {
		if c.Label == "" || (c.Point != "dashboard" && c.Point != "file-menu") || !actions[c.Action] {
			return fmt.Errorf("插件扩展位置无效")
		}
	}
	if d.Service != nil {
		c := d.Service
		if fields[c.URLField].Type != "url" || fields[c.TokenField].Type != "secret" {
			return fmt.Errorf("插件服务连接字段无效")
		}
		for _, key := range c.Credentials {
			if key == c.TokenField || fields[key].Type != "secret" {
				return fmt.Errorf("插件服务凭据字段无效")
			}
		}
	}
	if d.Resources != nil {
		p := d.Resources
		if !actions[p.SearchAction] || !actions[p.UnlockAction] || p.SearchAction == p.UnlockAction || p.FilterAction != "" && (!actions[p.FilterAction] || p.FilterAction == p.UnlockAction) {
			return fmt.Errorf("插件资源动作无效")
		}
		for _, a := range d.Actions {
			// Encrypted POST queries use run + idempotent; ordinary probe actions stay read-only.
			searchOK := !a.Danger && (a.Kind == "probe" || a.Kind == "run" && a.Idempotent)
			if (a.ID == p.SearchAction || a.ID == p.FilterAction) && !searchOK || a.ID == p.UnlockAction && (!a.Danger || a.Kind != "run") {
				return fmt.Errorf("插件资源动作类型无效")
			}
		}
	}
	if d.Browser != nil {
		if len(d.Browser.Hosts) == 0 || len(d.Browser.Hosts) > 32 {
			return fmt.Errorf("插件浏览器站点无效")
		}
		for _, host := range d.Browser.Hosts {
			if host == "" || len(host) > 253 || strings.ContainsAny(host, "/\\?#@ \t\r\n*") {
				return fmt.Errorf("插件浏览器站点无效")
			}
		}
		for _, key := range d.Browser.Credentials {
			if fields[key].Type != "secret" {
				return fmt.Errorf("插件浏览器凭据字段无效")
			}
		}
	}
	if d.Storage != nil {
		if d.Runtime != "wasm" {
			return fmt.Errorf("网盘插件需要 WASM 运行时")
		}
		if err := d.Storage.Validate(); err != nil {
			return err
		}
		for _, rule := range d.Storage.ShareLinks {
			declared := false
			for _, host := range d.HTTPHosts {
				declared = declared || strings.EqualFold(host, rule.Host)
			}
			if !declared {
				return fmt.Errorf("分享域名需要声明 HTTP 访问权限")
			}
		}
		found := false
		for _, p := range d.Permissions {
			found = found || p == "storage.provider"
		}
		if !found {
			return fmt.Errorf("网盘插件缺少驱动权限")
		}
	}
	return nil
}
