package pluginapi

import (
	"math"
	"testing"
)

func TestStorageFieldSchemaRejectsInvalidDefinitions(t *testing.T) {
	tests := map[string][]Field{
		"self dependency":    {{Key: "v", Label: "版本", Type: "text", DependsOn: []string{"v"}}},
		"cycle":              {{Key: "a", Label: "A", Type: "text", DependsOn: []string{"b"}}, {Key: "b", Label: "B", Type: "text", DependsOn: []string{"a"}}},
		"unknown dependency": {{Key: "v", Label: "版本", Type: "text", DependsOn: []string{"missing"}}},
		"secret dependency":  {{Key: "token", Label: "密钥", Type: "secret"}, {Key: "v", Label: "版本", Type: "select", OptionsSource: "versions", DependsOn: []string{"token"}}},
		"invalid default":    {{Key: "v", Label: "版本", Type: "select", Options: []Option{{Value: "v1", Label: "版本一"}}, Default: "missing"}},
		"duplicate options":  {{Key: "v", Label: "版本", Type: "select", Options: []Option{{Value: "v1", Label: "版本一"}, {Value: "v1", Label: "版本二"}}}},
		"required readonly":  {{Key: "v", Label: "服务版本", Type: "text", ReadOnly: true, Required: true}},
		"readonly dynamic":   {{Key: "v", Label: "版本", Type: "select", ReadOnly: true, OptionsSource: "versions"}},
		"mistyped condition": {{Key: "flag", Label: "开关", Type: "boolean"}, {Key: "v", Label: "版本", Type: "text", VisibleWhen: &Condition{Field: "flag", Equals: "true"}}},
		"secret default":     {{Key: "v", Label: "密钥", Type: "secret", Default: "secret"}},
		"nonfinite default":  {{Key: "v", Label: "次数", Type: "number", Default: math.Inf(1)}},
	}
	for name, fields := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateStorageFields(fields, false); err == nil {
				t.Fatal("invalid schema accepted")
			}
		})
	}
	if err := ValidateStorageFields([]Field{{Key: "token", Label: "密钥", Type: "secret"}}, true); err == nil {
		t.Fatal("task secret accepted")
	}
}

func TestStorageStringConditionsAndZeroDefaults(t *testing.T) {
	fields := []Field{{Key: "version", Label: "版本", Type: "select", Default: "v2", Options: []Option{{Value: "v1", Label: "旧版"}, {Value: "v2", Label: "新版"}}}, {Key: "count", Label: "次数", Type: "number", Default: 0, VisibleWhen: &Condition{Field: "version", Equals: "v2"}}, {Key: "enabled", Label: "开关", Type: "boolean", Default: false}}
	if err := ValidateStorageFields(fields, false); err != nil {
		t.Fatal(err)
	}
	values := StorageDefaults(fields, nil)
	if values["count"] != 0 || values["enabled"] != false || !StorageFieldVisible(fields, fields[1], values) {
		t.Fatal(values)
	}
	values["version"] = "v1"
	if _, ok := ActiveStorageValues(fields, values)["count"]; ok {
		t.Fatal("hidden parameter sent")
	}
	if !StorageEqual(float64(0), 0) {
		t.Fatal("JSON numeric equality inconsistent")
	}
}
