package pluginapi

import "testing"

func TestSDKRuntimeCredentialsAndEncryptedResourceActions(t *testing.T) {
	d := Definition{SchemaVersion: 2, APIVersion: "1", ID: "sdk-resource", Name: "SDK", Version: "1", Runtime: "wasm", ConfigVersion: 1,
		Fields:    []Field{{Key: "key", Label: "Key", Type: "secret", RuntimeAccess: true}},
		Actions:   []Action{{ID: "search", Label: "Search", Kind: "run", Idempotent: true}, {ID: "unlock", Label: "Unlock", Kind: "run", Danger: true, Confirmation: "Unlock this resource?"}},
		Resources: &ResourceProvider{SearchAction: "search", UnlockAction: "unlock"}}
	if e := d.Validate(); e != nil {
		t.Fatal(e)
	}
	d.Fields[0].Type = "text"
	if e := d.Validate(); e == nil {
		t.Fatal("runtime credential flag accepted on plain text")
	}
	d.Fields[0].Type = "secret"
	d.Actions[0].Idempotent = false
	if e := d.Validate(); e == nil {
		t.Fatal("non-idempotent search accepted")
	}
	d.Actions[0].Idempotent = true
	d.Actions[0].Danger = true
	if e := d.Validate(); e == nil {
		t.Fatal("dangerous search accepted")
	}
}

func TestResourceFilterActionContract(t *testing.T) {
	d := Definition{SchemaVersion: 2, APIVersion: "1", ID: "filtered-resource", Name: "Filter", Version: "1", Runtime: "wasm", ConfigVersion: 1,
		Actions:   []Action{{ID: "search", Label: "Search", Kind: "probe"}, {ID: "query", Label: "Filter", Kind: "run", Idempotent: true}, {ID: "unlock", Label: "Unlock", Kind: "run", Danger: true, Confirmation: "Unlock?"}},
		Resources: &ResourceProvider{SearchAction: "search", FilterAction: "query", UnlockAction: "unlock"}}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"missing", "unlock"} {
		d.Resources.FilterAction = id
		if d.Validate() == nil {
			t.Fatal("invalid filter accepted", id)
		}
	}
	d.Resources.FilterAction = "query"
	d.Actions[1].Idempotent = false
	if d.Validate() == nil {
		t.Fatal("writing filter accepted")
	}
	d.Actions[1].Idempotent = true
	d.Actions[1].Danger = true
	if d.Validate() == nil {
		t.Fatal("dangerous filter accepted")
	}
}

func TestActionInputConditionsAndRanges(t *testing.T) {
	min, max, step := 0.0, 10000.0, 1.0
	a := Action{ID: "query", InputFields: []Field{{Key: "media_type", Label: "Media", Type: "text"}, {Key: "season", Label: "Season", Type: "number", Min: &min, Max: &max, Step: &step, VisibleWhen: &Condition{Field: "media_type", Equals: "tv"}}}}
	if err := validateActionInputs(a); err != nil {
		t.Fatal(err)
	}
	a.InputFields[1].VisibleWhen.Field = "missing"
	if validateActionInputs(a) == nil {
		t.Fatal("undefined condition accepted")
	}
	a.InputFields[1].VisibleWhen.Field = "season"
	if validateActionInputs(a) == nil {
		t.Fatal("self condition accepted")
	}
	a.InputFields[1].VisibleWhen = nil
	step = 0
	if validateActionInputs(a) == nil {
		t.Fatal("zero step accepted")
	}
	step, min = 1, 10001
	if validateActionInputs(a) == nil {
		t.Fatal("reversed limits accepted")
	}
}
