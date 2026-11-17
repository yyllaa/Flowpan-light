package pluginapi

import (
	"encoding/json"
	"fmt"
	"math"
)

// FilePickerAccount binds a path field to a declared source field. The data
// binding selects a directory for this plugin. The host derives automatic roots
// from explicitly saved values, within the user's existing account grants.
func (f Field) FilePickerAccount() (string, error) {
	raw, ok := f.Extensions["flowpan.files"]
	if !ok {
		return "", nil
	}
	var binding struct {
		AccountField string `json:"account_field"`
	}
	if err := json.Unmarshal(raw, &binding); err != nil || f.Type != "path" || !identifier.MatchString(binding.AccountField) {
		return "", fmt.Errorf("插件目录选择声明无效：%s", f.Key)
	}
	return binding.AccountField, nil
}

func (d Definition) validateConfigFileFields() error {
	fields := map[string]Field{}
	for _, field := range d.Fields {
		fields[field.Key] = field
	}
	for _, field := range d.Fields {
		if field.Type == "account-list" {
			for _, bound := range []*float64{field.Min, field.Max} {
				if bound != nil && (math.IsNaN(*bound) || math.IsInf(*bound, 0) || *bound < 0 || *bound > 50 || math.Trunc(*bound) != *bound) {
					return fmt.Errorf("插件来源数量声明无效：%s", field.Key)
				}
			}
			if field.Min != nil && field.Max != nil && *field.Min > *field.Max {
				return fmt.Errorf("插件来源数量声明无效：%s", field.Key)
			}
		}
		key, err := field.FilePickerAccount()
		if err != nil {
			return err
		}
		if key != "" && fields[key].Type != "account-list" {
			return fmt.Errorf("插件目录来源字段无效：%s", field.Key)
		}
	}
	return nil
}
