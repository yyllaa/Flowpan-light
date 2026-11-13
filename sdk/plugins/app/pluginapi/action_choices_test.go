package pluginapi

import (
	"encoding/json"
	"testing"
)

func choiceDefinition() Definition {
	return Definition{SchemaVersion: SchemaVersion, APIVersion: APIVersion, ID: "choice-fixture", Name: "选择", Version: "1.0.0", ConfigVersion: 1, Runtime: "builtin", Actions: []Action{
		{ID: "normal", Label: "执行", Kind: "run", Choices: []Option{{Value: "normal", Label: "普通"}, {Value: "alternate", Label: "另一种"}}},
		{ID: "alternate", Label: "另一种", Kind: "run", Placement: "tools", Danger: true, Confirmation: "确认执行？"},
	}}
}

func TestActionChoicesRoundTripAndLegacyCompatibility(t *testing.T) {
	d := choiceDefinition()
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(d)
	var loaded Definition
	if err := json.Unmarshal(b, &loaded); err != nil || len(loaded.Actions[0].Choices) != 2 || !loaded.Actions[1].Danger {
		t.Fatal("choices or target confirmation lost", err)
	}
	d.Actions[0].Choices = nil
	if err := d.Validate(); err != nil {
		t.Fatal("legacy action rejected", err)
	}
}

func TestActionChoicesRejectBrokenReferencesAndUnsupportedForms(t *testing.T) {
	for name, mutate := range map[string]func(*Definition){
		"unknown":   func(d *Definition) { d.Actions[0].Choices[1].Value = "missing" },
		"duplicate": func(d *Definition) { d.Actions[0].Choices[1].Value = "normal" },
		"label":     func(d *Definition) { d.Actions[0].Choices[1].Label = "" },
		"default":   func(d *Definition) { d.Actions[0].Choices[0].Value = "alternate" },
		"single":    func(d *Definition) { d.Actions[0].Choices = d.Actions[0].Choices[:1] },
		"placement": func(d *Definition) { d.Actions[1].Placement = "" },
		"input":     func(d *Definition) { d.Actions[1].InputFields = []Field{{Key: "name", Label: "名称", Type: "text"}} },
		"nested": func(d *Definition) {
			d.Actions[1].Choices = []Option{{Value: "alternate", Label: "另一种"}, {Value: "normal", Label: "普通"}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			d := choiceDefinition()
			mutate(&d)
			if d.Validate() == nil {
				t.Fatal("invalid choices accepted")
			}
		})
	}
}

func TestActionChoiceConfigBinding(t *testing.T) {
	fixture := func() Definition {
		d := choiceDefinition()
		d.Fields = []Field{{Key: "mode", Label: "方式", Type: "select", Default: "normal", Options: append([]Option{}, d.Actions[0].Choices...)}}
		d.Actions[0].ChoiceField = "mode"
		return d
	}
	d := fixture()
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(d)
	var loaded Definition
	if err := json.Unmarshal(b, &loaded); err != nil || loaded.Actions[0].ChoiceField != "mode" {
		t.Fatal("binding lost in persisted definition", err)
	}
	for name, mutate := range map[string]func(*Definition){
		"missing":  func(d *Definition) { d.Actions[0].ChoiceField = "missing" },
		"readonly": func(d *Definition) { d.Fields[0].ReadOnly = true },
		"type":     func(d *Definition) { d.Fields[0].Type = "text" },
		"hidden": func(d *Definition) {
			d.Fields = append(d.Fields, Field{Key: "enabled", Label: "开关", Type: "boolean"})
			d.Fields[0].VisibleWhen = &Condition{Field: "enabled", Equals: true}
		},
		"option":     func(d *Definition) { d.Fields[0].Options[1].Value = "missing" },
		"count":      func(d *Definition) { d.Fields[0].Options = d.Fields[0].Options[:1] },
		"default":    func(d *Definition) { d.Fields[0].Default = "missing" },
		"no-choices": func(d *Definition) { d.Actions[0].Choices = nil },
	} {
		t.Run(name, func(t *testing.T) {
			d := fixture()
			mutate(&d)
			if d.Validate() == nil {
				t.Fatal("invalid config binding accepted")
			}
		})
	}
}
