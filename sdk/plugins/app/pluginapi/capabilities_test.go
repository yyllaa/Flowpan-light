package pluginapi

import "testing"

func TestCapabilitySchemasRejectInvalidDataAndUnboundedContracts(t *testing.T) {
	s := ValueSchema{Type: "object", Required: []string{"title", "items"}, Properties: map[string]ValueSchema{"title": {Type: "string", Enum: []string{"music", "film"}}, "items": {Type: "array", Items: &ValueSchema{Type: "integer"}}}}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{
		map[string]any{"title": "music", "items": []int{1, 2}},
	} {
		if err := s.Check(value); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []any{nil, map[string]any{"title": "music"}, map[string]any{"title": "other", "items": []int{1}}, map[string]any{"title": "film", "items": []float64{1.5}}, map[string]any{"title": "music", "items": []int{1}, "surprise": true}} {
		if err := s.Check(value); err == nil {
			t.Fatalf("accepted %#v", value)
		}
	}
	for _, schema := range []ValueSchema{{Type: "array"}, {Type: "object", Required: []string{"absent"}}, {Type: "string", Properties: map[string]ValueSchema{"x": {Type: "number"}}}, {Type: "unknown"}} {
		if err := schema.Validate(); err == nil {
			t.Fatalf("accepted schema %#v", schema)
		}
	}
	deep := ValueSchema{Type: "string"}
	for range 8 {
		deep = ValueSchema{Type: "array", Items: &deep}
	}
	if err := deep.Validate(); err == nil {
		t.Fatal("unbounded contract")
	}
}

func TestCapabilityDeclarationsBindOnlyExistingActions(t *testing.T) {
	d := Definition{SchemaVersion: 2, APIVersion: "1", ID: "provider", Version: "1.0.0", Name: "provider", Runtime: "wasm", ConfigVersion: 1, Actions: []Action{{ID: "format", Label: "format", Kind: "probe"}}, Provides: []CapabilityProvider{{ID: "format", Interface: "text.format", Version: 1, Action: "missing", Input: ValueSchema{Type: "object"}, Output: ValueSchema{Type: "object"}}}}
	if err := d.Validate(); err == nil {
		t.Fatal("missing handler accepted")
	}
	d.Provides[0].Action = "format"
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	d.Uses = []CapabilityUse{{ID: "next", Interface: "new.interface.without.host.changes", Version: 1}}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	d.Uses = append(d.Uses, d.Uses[0])
	if err := d.Validate(); err == nil {
		t.Fatal("duplicate alias accepted")
	}
	d.Uses = nil
	other := d.Provides[0]
	other.ID = "other"
	other.Output.AdditionalProperties = true
	d.Provides = append(d.Provides, other)
	if err := d.Validate(); err == nil {
		t.Fatal("conflicting contracts inside one package accepted")
	}
}
