package pluginapi

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
)

// Storage fields are data, never component code or layout instructions.
func ValidateStorageFields(fields []Field, task bool) error {
	if len(fields) > 48 {
		return fmt.Errorf("网盘字段数量超出限制")
	}
	byKey := map[string]Field{}
	for _, f := range fields {
		if !identifier.MatchString(f.Key) || f.Label == "" || len(f.Label) > 160 || len(f.Hint) > 2000 || len(f.Group) > 120 {
			return fmt.Errorf("网盘账号字段无效")
		}
		if _, exists := byKey[f.Key]; exists {
			return fmt.Errorf("网盘账号字段重复：%s", f.Key)
		}
		switch f.Type {
		case "text", "secret", "url", "number", "boolean", "select":
		default:
			return fmt.Errorf("网盘账号字段类型无效")
		}
		if f.ReadOnly && f.Required {
			return fmt.Errorf("只读字段不能声明为用户必填")
		}
		if task && f.Type == "secret" || f.Type == "secret" && (f.ReadOnly || f.Default != nil) {
			return fmt.Errorf("网盘密钥字段声明无效")
		}
		if f.OptionsSource != "" && (f.Type != "select" || !identifier.MatchString(f.OptionsSource) || f.ReadOnly) {
			return fmt.Errorf("网盘动态选项声明无效")
		}
		if err := ValidateStorageOptions(f.Options); err != nil {
			return err
		}
		if f.Type == "select" && len(f.Options) == 0 && f.OptionsSource == "" {
			return fmt.Errorf("网盘账号选项为空")
		}
		if f.Default != nil {
			if err := ValidateStorageValue(f, f.Default); err != nil {
				return err
			}
		}
		byKey[f.Key] = f
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	var walk func(string) error
	walk = func(key string) error {
		if visiting[key] {
			return fmt.Errorf("网盘字段依赖循环")
		}
		if visited[key] {
			return nil
		}
		visiting[key] = true
		f := byKey[key]
		deps := append([]string{}, f.DependsOn...)
		if f.VisibleWhen != nil {
			dep, ok := byKey[f.VisibleWhen.Field]
			if !ok || dep.Type == "secret" {
				return fmt.Errorf("网盘字段显示条件无效")
			}
			if err := ValidateStorageValue(dep, f.VisibleWhen.Equals); err != nil {
				return err
			}
			deps = append(deps, f.VisibleWhen.Field)
		}
		seen := map[string]bool{}
		for _, dep := range deps {
			d, ok := byKey[dep]
			if !ok || d.Type == "secret" || dep == key {
				return fmt.Errorf("网盘字段依赖无效")
			}
			if seen[dep] {
				continue
			}
			seen[dep] = true
			if err := walk(dep); err != nil {
				return err
			}
		}
		delete(visiting, key)
		visited[key] = true
		return nil
	}
	for key := range byKey {
		if err := walk(key); err != nil {
			return err
		}
	}
	return nil
}

func ValidateStorageOptions(options []Option) error {
	if len(options) > 200 {
		return fmt.Errorf("网盘账号选项超出限制")
	}
	seen := map[string]bool{}
	for _, o := range options {
		if o.Value == "" || len(o.Value) > 512 || o.Label == "" || len(o.Label) > 300 || seen[o.Value] {
			return fmt.Errorf("网盘账号选项无效")
		}
		seen[o.Value] = true
	}
	return nil
}

func ValidateStorageValue(f Field, value any) error {
	valid := false
	switch f.Type {
	case "boolean":
		_, valid = value.(bool)
	case "number":
		switch v := value.(type) {
		case float64:
			valid = !math.IsNaN(v) && !math.IsInf(v, 0)
		case int, int64:
			valid = true
		}
	default:
		if s, ok := value.(string); ok && len(s) <= 16384 {
			valid = true
			if f.Type == "select" && s != "" && f.OptionsSource == "" {
				valid = false
				for _, o := range f.Options {
					if o.Value == s {
						valid = true
					}
				}
			}
		}
	}
	if !valid {
		return fmt.Errorf("%s 格式或选项无效", f.Label)
	}
	return nil
}

func StorageDefaults(fields []Field, values map[string]any) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		if f.Default != nil {
			out[f.Key] = f.Default
		}
	}
	for k, v := range values {
		out[k] = v
	}
	return out
}

func StorageEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return reflect.DeepEqual(x, y)
}

func StorageFieldVisible(fields []Field, f Field, values map[string]any) bool {
	if f.VisibleWhen == nil {
		return true
	}
	for _, dep := range fields {
		if dep.Key == f.VisibleWhen.Field {
			return StorageFieldVisible(fields, dep, values) && StorageEqual(values[dep.Key], f.VisibleWhen.Equals)
		}
	}
	return false
}

func ActiveStorageValues(fields []Field, values map[string]any) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		if StorageFieldVisible(fields, f, values) {
			if v, ok := values[f.Key]; ok {
				out[f.Key] = v
			}
		}
	}
	return out
}
