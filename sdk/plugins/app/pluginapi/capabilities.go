package pluginapi

import (
	"encoding/json"
	"fmt"
	"math"
)

// CapabilityProvider exports one versioned interface through an ordinary action.
// Interface versions describe data and behavior independently of package versions.
type CapabilityProvider struct {
	ID        string      `json:"id"`
	Interface string      `json:"interface"`
	Version   int         `json:"version"`
	Action    string      `json:"action"`
	Input     ValueSchema `json:"input"`
	Output    ValueSchema `json:"output"`
}

type CapabilityUse struct {
	ID        string `json:"id"`
	Interface string `json:"interface"`
	Version   int    `json:"version"`
	Optional  bool   `json:"optional,omitempty"`
}

type CapabilityProviderInfo struct {
	CapabilityProvider
	Key           string `json:"key"`
	Plugin        string `json:"plugin"`
	Name          string `json:"name"`
	PluginVersion string `json:"plugin_version"`
	Package       string `json:"package"`
	Enabled       bool   `json:"enabled"`
	ReadOnly      bool   `json:"read_only"`
}

type CapabilityBindingStatus struct {
	CapabilityUse
	Provider string                   `json:"provider"`
	Ready    bool                     `json:"ready"`
	Reason   string                   `json:"reason,omitempty"`
	Options  []CapabilityProviderInfo `json:"options"`
}

type CapabilityCall struct {
	Use   string         `json:"use"`
	Input map[string]any `json:"input,omitempty"`
	Key   string         `json:"key,omitempty"`
	ID    string         `json:"id,omitempty"`
}

// CapabilityInvocation is created by the host, never accepted from guest input.
// It is persisted with asynchronous jobs and pins both participants and the route.
type CapabilityInvocation struct {
	Caller        string                `json:"caller"`
	CallerPackage string                `json:"caller_package"`
	Use           string                `json:"use"`
	Provider      CapabilityProvider    `json:"provider"`
	Package       string                `json:"package"`
	Chain         []string              `json:"chain"`
	CallerScope   Scope                 `json:"caller_scope"`
	ReadOnly      bool                  `json:"read_only"`
	Principals    []CapabilityPrincipal `json:"principals"`
}

type CapabilityPrincipal struct {
	Plugin   string `json:"plugin"`
	Package  string `json:"package"`
	Use      string `json:"use"`
	Provider string `json:"provider"`
	Scope    Scope  `json:"scope"`
}

func capabilityTools() []Tool {
	return []Tool{
		tool("capabilities.list", "host.read", "插件能力", "查询本插件声明的能力依赖、绑定与可用提供者，不执行插件", "", false, false),
		tool("capabilities.call", "capabilities.call", "插件能力", "调用已绑定的插件接口，继承取消、调用预算和授权范围", "use:string input?:object", false, false),
		tool("capabilities.submit", "capabilities.call", "插件能力", "提交已绑定接口的后台任务，使用稳定幂等键", "use:string input?:object key:string", true, false),
		tool("capabilities.task", "capabilities.call", "插件能力", "查询由本插件提交的能力任务", "id:string", false, false),
		tool("capabilities.cancel", "capabilities.call", "插件能力", "取消由本插件提交且声明可取消的能力任务", "id:string", true, false),
	}
}

// ValueSchema is a deliberately bounded JSON Schema subset. Unsupported keywords
// are rejected by the package decoder rather than silently ignored.
type ValueSchema struct {
	Type                 string                 `json:"type"`
	Properties           map[string]ValueSchema `json:"properties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	Items                *ValueSchema           `json:"items,omitempty"`
	Enum                 []string               `json:"enum,omitempty"`
	AdditionalProperties bool                   `json:"additional_properties,omitempty"`
}

func (s ValueSchema) Validate() error {
	budget := 0
	return s.validate(0, &budget)
}

func (s ValueSchema) validate(depth int, budget *int) error {
	*budget++
	if depth > 6 || *budget > 128 || len(s.Required) > 64 || len(s.Enum) > 64 {
		return fmt.Errorf("能力数据结构超过上限")
	}
	switch s.Type {
	case "object", "array", "string", "number", "integer", "boolean", "null":
	default:
		return fmt.Errorf("能力数据类型无效：%s", s.Type)
	}
	if s.Type != "object" && (len(s.Properties) > 0 || len(s.Required) > 0 || s.AdditionalProperties) || s.Type != "array" && s.Items != nil || s.Type != "string" && len(s.Enum) > 0 {
		return fmt.Errorf("能力数据结构与类型不一致")
	}
	for key, child := range s.Properties {
		if !identifier.MatchString(key) || len(key) > 128 {
			return fmt.Errorf("能力数据字段无效：%s", key)
		}
		if err := child.validate(depth+1, budget); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	for _, key := range s.Required {
		if _, ok := s.Properties[key]; !ok || seen[key] {
			return fmt.Errorf("能力必填字段无效：%s", key)
		}
		seen[key] = true
	}
	if s.Type == "array" {
		if s.Items == nil {
			return fmt.Errorf("能力数组缺少元素结构")
		}
		return s.Items.validate(depth+1, budget)
	}
	return nil
}

func (s ValueSchema) Check(value any) error {
	b, err := json.Marshal(value)
	if err != nil || len(b) > MaxHostResponse {
		return fmt.Errorf("能力数据超过上限")
	}
	var normalized any
	if err = json.Unmarshal(b, &normalized); err != nil {
		return err
	}
	return s.check(normalized, "data", 0)
}

func (s ValueSchema) check(value any, path string, depth int) error {
	if depth > 32 {
		return fmt.Errorf("能力数据层级超过上限")
	}
	bad := func() error { return fmt.Errorf("能力数据不符合结构：%s (%s)", path, s.Type) }
	switch s.Type {
	case "object":
		m, ok := value.(map[string]any)
		if !ok {
			return bad()
		}
		for _, key := range s.Required {
			if _, ok := m[key]; !ok {
				return fmt.Errorf("能力数据缺少字段：%s.%s", path, key)
			}
		}
		for key, v := range m {
			child, ok := s.Properties[key]
			if !ok {
				if !s.AdditionalProperties {
					return bad()
				}
				continue
			}
			if err := child.check(v, path+"."+key, depth+1); err != nil {
				return err
			}
		}
	case "array":
		a, ok := value.([]any)
		if !ok || s.Items == nil {
			return bad()
		}
		for _, v := range a {
			if err := s.Items.check(v, path+"[]", depth+1); err != nil {
				return err
			}
		}
	case "string":
		v, ok := value.(string)
		if !ok {
			return bad()
		}
		if len(s.Enum) > 0 {
			found := false
			for _, choice := range s.Enum {
				found = found || choice == v
			}
			if !found {
				return bad()
			}
		}
	case "number", "integer":
		v, ok := value.(float64)
		if !ok || s.Type == "integer" && math.Trunc(v) != v {
			return bad()
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return bad()
		}
	case "null":
		if value != nil {
			return bad()
		}
	default:
		return bad()
	}
	return nil
}

func (d *Definition) validateCapabilities() error {
	if len(d.Provides) > 32 || len(d.Uses) > 32 {
		return fmt.Errorf("插件能力声明超过上限")
	}
	seen := map[string]bool{}
	contracts := map[string]string{}
	for _, p := range d.Provides {
		if !identifier.MatchString(p.ID) || !identifier.MatchString(p.Interface) || len(p.Interface) > 128 || p.Version < 1 || p.Version > 1000000 || seen[p.ID] {
			return fmt.Errorf("插件提供能力声明无效：%s", p.ID)
		}
		seen[p.ID] = true
		var action *Action
		for _, a := range d.Actions {
			if a.ID == p.Action {
				a := a
				action = &a
			}
		}
		if action == nil || action.RequiresResult != nil {
			return fmt.Errorf("能力执行动作无效：%s", p.ID)
		}
		if p.Input.Type != "object" || p.Output.Type != "object" {
			return fmt.Errorf("能力输入输出须为对象：%s", p.ID)
		}
		if err := p.Input.Validate(); err != nil {
			return err
		}
		if err := p.Output.Validate(); err != nil {
			return err
		}
		key := fmt.Sprintf("%s/%d", p.Interface, p.Version)
		raw, _ := json.Marshal([]any{p.Input, p.Output, action.Kind == "probe"})
		if previous, ok := contracts[key]; ok && previous != string(raw) {
			return fmt.Errorf("同一能力接口版本的输入输出或读写语义不一致：%s", key)
		}
		contracts[key] = string(raw)
	}
	seen = map[string]bool{}
	for _, u := range d.Uses {
		if !identifier.MatchString(u.ID) || !identifier.MatchString(u.Interface) || len(u.Interface) > 128 || u.Version < 1 || u.Version > 1000000 || seen[u.ID] {
			return fmt.Errorf("插件依赖能力声明无效：%s", u.ID)
		}
		seen[u.ID] = true
	}
	return nil
}
