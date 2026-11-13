package pluginapi

import (
	"encoding/json"
	"fmt"
	"math"
)

// SDKAccess binds a private host SDK to this plugin's own encrypted credential.
// The guest receives business data, never the credential or SDK implementation.
type SDKAccess struct {
	Provider   string `json:"provider"`
	Credential string `json:"credential"`
}

type SDKRequest struct {
	Provider  string         `json:"provider"`
	Operation string         `json:"operation"`
	Input     map[string]any `json:"input,omitempty"`
}

type SDKFailure struct {
	Status   int    `json:"status,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message"`
	Verified bool   `json:"verified,omitempty"`
}

func (e *SDKFailure) Error() string { return e.Message }

type SDKResponse struct {
	Account string          `json:"account,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	Failure *SDKFailure     `json:"failure,omitempty"`
}

func (d Definition) validateSDKs(fields map[string]Field) error {
	seen := map[string]bool{}
	for _, binding := range d.SDKs {
		if !identifier.MatchString(binding.Provider) || seen[binding.Provider] || fields[binding.Credential].Type != "secret" {
			return fmt.Errorf("宿主 SDK 凭据绑定无效：%s", binding.Provider)
		}
		seen[binding.Provider] = true
	}
	return nil
}

func validateActionInputs(a Action) error {
	if a.Placement != "" && a.Placement != "tools" {
		return fmt.Errorf("插件动作位置无效：%s", a.ID)
	}
	seen := map[string]bool{}
	for _, f := range a.InputFields {
		if !identifier.MatchString(f.Key) || f.Label == "" || seen[f.Key] || f.ReadOnly || f.RuntimeAccess {
			return fmt.Errorf("插件动作参数无效：%s", a.ID)
		}
		switch f.Type {
		case "text", "number", "select", "boolean", "json", "textarea", "string-list", "multi-select":
		default:
			return fmt.Errorf("插件动作参数类型无效：%s", f.Key)
		}
		seen[f.Key] = true
	}
	groups := map[string]bool{}
	for _, g := range a.InputGroups {
		if !identifier.MatchString(g.ID) || g.Label == "" || groups[g.ID] {
			return fmt.Errorf("插件动作分组无效：%s", a.ID)
		}
		groups[g.ID] = true
	}
	if m := a.MediaInput; m != nil {
		types := map[string]string{}
		for _, f := range a.InputFields {
			types[f.Key] = f.Type
		}
		if types[m.IDField] != "number" || types[m.TypeField] != "select" || m.IDField == m.TypeField {
			return fmt.Errorf("插件作品选择器绑定无效：%s", a.ID)
		}
	}
	for _, f := range a.InputFields {
		if len(groups) > 0 && f.Group != "" && !groups[f.Group] {
			return fmt.Errorf("插件动作分组不存在：%s", f.Key)
		}
		if f.Type == "multi-select" || f.Type == "select" {
			values := map[string]bool{}
			for _, o := range f.Options {
				if o.Label == "" || values[o.Value] {
					return fmt.Errorf("插件动作选项无效：%s", f.Key)
				}
				values[o.Value] = true
			}
		}
		if c := f.VisibleWhen; c != nil {
			if !seen[c.Field] || c.Field == f.Key {
				return fmt.Errorf("插件动作参数显示条件无效：%s", f.Key)
			}
			switch c.Equals.(type) {
			case string, bool, float64, int:
			default:
				return fmt.Errorf("插件动作参数显示条件无效：%s", f.Key)
			}
		}
		for _, bound := range []*float64{f.Min, f.Max, f.Step} {
			if bound != nil && (f.Type != "number" || math.IsNaN(*bound) || math.IsInf(*bound, 0)) {
				return fmt.Errorf("插件动作数字范围无效：%s", f.Key)
			}
		}
		if f.Min != nil && f.Max != nil && *f.Min > *f.Max || f.Step != nil && *f.Step <= 0 {
			return fmt.Errorf("插件动作数字范围无效：%s", f.Key)
		}
	}
	return nil
}
