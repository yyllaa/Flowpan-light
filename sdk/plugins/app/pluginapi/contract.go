package pluginapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// HostRequirements describes interfaces, not plugin business logic or grants.
// Feature versions change only when a host interface gains new semantics.
type HostRequirements struct {
	APIVersion string           `json:"api_version,omitempty"`
	Features   map[string]int   `json:"features,omitempty"`
	Tools      []string         `json:"tools,omitempty"`
	Limits     map[string]int64 `json:"limits,omitempty"`
}

type HostContract struct {
	HostVersion string           `json:"host_version"`
	APIVersion  string           `json:"api_version"`
	Features    map[string]int   `json:"features"`
	Tools       []string         `json:"tools"`
	Limits      map[string]int64 `json:"limits"`
}

type RequirementIssue struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Required string `json:"required"`
	Actual   string `json:"actual"`
}

type CompatibilityReport struct {
	Compatible      bool               `json:"compatible"`
	HostVersion     string             `json:"host_version"`
	Missing         []RequirementIssue `json:"missing"`
	OptionalMissing []RequirementIssue `json:"optional_missing"`
}

type CompatibilityError struct{ Report CompatibilityReport }

func (e *CompatibilityError) Error() string {
	parts := make([]string, 0, len(e.Report.Missing))
	for _, issue := range e.Report.Missing {
		parts = append(parts, fmt.Sprintf("%s 要求 %s，当前 %s", issue.Name, issue.Required, issue.Actual))
	}
	return "主程序能力不满足：" + strings.Join(parts, "；") + "。请更新主程序，或使用兼容的插件版本"
}

func (r CompatibilityReport) Err() error {
	if r.Compatible {
		return nil
	}
	return &CompatibilityError{Report: r}
}

func CurrentHostContract(hostVersion string) HostContract {
	c := HostContract{HostVersion: hostVersion, APIVersion: APIVersion,
		Features: map[string]int{
			"manifest.requirements": 1, "manifest.extensions": 1, "plugins.capabilities": 1,
			"jobs.continuation": 1, "http.response.base64": 1,
			"media.cover-artifact": 1, "media.lyrics": 1,
			"state.indexed-get": 1, "files.verified-publish": 1,
			"ui.pages": 1, "actions.dynamic-options": 1, "tasks.activity": 1,
		}, Tools: []string{}, Limits: map[string]int64{}}
	for _, t := range Tools() {
		c.Tools = append(c.Tools, t.Name)
	}
	sort.Strings(c.Tools)
	b, _ := json.Marshal(RuntimeLimits())
	_ = json.Unmarshal(b, &c.Limits)
	return c
}

func (r *HostRequirements) Validate() error {
	if r == nil {
		return nil
	}
	if len(r.APIVersion) > 32 || len(r.Features) > 64 || len(r.Tools) > 256 || len(r.Limits) > 64 {
		return fmt.Errorf("插件能力声明超过上限")
	}
	for name, v := range r.Features {
		if !identifier.MatchString(name) || v < 1 || v > 1000000 {
			return fmt.Errorf("插件能力声明无效：%s", name)
		}
	}
	seen := map[string]bool{}
	for _, name := range r.Tools {
		if !identifier.MatchString(name) || seen[name] {
			return fmt.Errorf("插件工具声明无效：%s", name)
		}
		seen[name] = true
	}
	for name, v := range r.Limits {
		if !identifier.MatchString(name) || v < 1 {
			return fmt.Errorf("插件运行上限声明无效：%s", name)
		}
	}
	return nil
}

func (h HostContract) Check(required, optional *HostRequirements, minHost string) CompatibilityReport {
	report := CompatibilityReport{Compatible: true, HostVersion: h.HostVersion, Missing: []RequirementIssue{}, OptionalMissing: []RequirementIssue{}}
	if minHost != "" && minHost != "1" && !hostVersionAtLeast(h.HostVersion, minHost) {
		report.Missing = append(report.Missing, RequirementIssue{Kind: "host_version", Name: "主程序版本", Required: minHost, Actual: h.HostVersion})
	}
	check := func(r *HostRequirements) []RequirementIssue {
		issues := []RequirementIssue{}
		if r == nil {
			return issues
		}
		if r.APIVersion != "" && r.APIVersion != h.APIVersion {
			issues = append(issues, RequirementIssue{Kind: "api_version", Name: "插件 API", Required: r.APIVersion, Actual: h.APIVersion})
		}
		for _, name := range sortedKeys(r.Features) {
			v := r.Features[name]
			if h.Features[name] < v {
				issues = append(issues, RequirementIssue{Kind: "feature", Name: name, Required: fmt.Sprint(v), Actual: fmt.Sprint(h.Features[name])})
			}
		}
		tools := append([]string(nil), r.Tools...)
		sort.Strings(tools)
		for _, name := range tools {
			found := false
			for _, t := range h.Tools {
				if t == name {
					found = true
					break
				}
			}
			if !found {
				issues = append(issues, RequirementIssue{Kind: "tool", Name: name, Required: "已实现", Actual: "未实现"})
			}
		}
		for _, name := range sortedKeys(r.Limits) {
			v := r.Limits[name]
			if h.Limits[name] < v {
				issues = append(issues, RequirementIssue{Kind: "limit", Name: name, Required: fmt.Sprint(v), Actual: fmt.Sprint(h.Limits[name])})
			}
		}
		return issues
	}
	report.Missing = append(report.Missing, check(required)...)
	report.OptionalMissing = check(optional)
	report.Compatible = len(report.Missing) == 0
	return report
}

func sortedKeys[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Extensions is a bounded, namespaced data envelope. Unknown entries remain
// data; they never grant permissions or enable host behavior implicitly.
type Extensions map[string]json.RawMessage

func (e Extensions) Normalize() error {
	if len(e) > 32 {
		return fmt.Errorf("插件扩展声明超过上限")
	}
	total := 0
	for name, raw := range e {
		if !identifier.MatchString(name) || !strings.Contains(name, ".") {
			return fmt.Errorf("插件扩展名称无效：%s", name)
		}
		total += len(name) + len(raw)
		if total > 64<<10 {
			return fmt.Errorf("插件扩展声明过大")
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		var value any
		if err := dec.Decode(&value); err != nil {
			return fmt.Errorf("插件扩展数据无效：%s", name)
		}
		if err := dec.Decode(&struct{}{}); err != io.EOF {
			return fmt.Errorf("插件扩展数据无效：%s", name)
		}
		canonical, err := json.Marshal(value)
		if err != nil {
			return err
		}
		e[name] = canonical
	}
	return nil
}

func (d Definition) normalizeExtensions() error {
	if err := d.Extensions.Normalize(); err != nil {
		return err
	}
	for _, f := range d.Fields {
		if err := f.Extensions.Normalize(); err != nil {
			return err
		}
	}
	for _, a := range d.Actions {
		if err := a.Extensions.Normalize(); err != nil {
			return err
		}
		for _, f := range a.InputFields {
			if err := f.Extensions.Normalize(); err != nil {
				return err
			}
		}
	}
	return nil
}

// min_host_version=1 is the original API-level alias. Dated and numeric
// minimums compare numerically; feature requirements handle same-day variants.
func hostVersionAtLeast(actual, minimum string) bool {
	parse := func(s string) []string {
		s = strings.TrimPrefix(strings.TrimSpace(s), "v")
		s = strings.SplitN(strings.SplitN(s, "+", 2)[0], "-", 2)[0]
		parts := strings.Split(s, ".")
		for i, p := range parts {
			if p == "" {
				return nil
			}
			for _, c := range p {
				if c < '0' || c > '9' {
					return nil
				}
			}
			p = strings.TrimLeft(p, "0")
			if p == "" {
				p = "0"
			}
			parts[i] = p
		}
		return parts
	}
	a, b := parse(actual), parse(minimum)
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	for len(a) < len(b) {
		a = append(a, "0")
	}
	for len(b) < len(a) {
		b = append(b, "0")
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return len(a[i]) > len(b[i])
		}
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return true
}
