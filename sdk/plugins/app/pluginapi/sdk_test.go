package pluginapi

import "testing"

func TestSDKBindingsAndActionInputsAreDeclarative(t *testing.T) {
	fields := map[string]Field{"key": {Key: "key", Type: "secret"}, "plain": {Key: "plain", Type: "text"}}
	valid := Definition{SDKs: []SDKAccess{{Provider: "dian115", Credential: "key"}}}
	if e := valid.validateSDKs(fields); e != nil {
		t.Fatal(e)
	}
	for _, bindings := range [][]SDKAccess{{{Provider: "dian115", Credential: "plain"}}, {{Provider: "dian115", Credential: "missing"}}, {{Provider: "dian115", Credential: "key"}, {Provider: "dian115", Credential: "key"}}} {
		if e := (Definition{SDKs: bindings}).validateSDKs(fields); e == nil {
			t.Fatal("invalid SDK binding accepted")
		}
	}
	if e := validateActionInputs(Action{ID: "query", Placement: "tools", InputFields: []Field{{Key: "payload", Label: "内容", Type: "json"}}}); e != nil {
		t.Fatal(e)
	}
	for _, a := range []Action{{ID: "a", Placement: "arbitrary"}, {ID: "a", InputFields: []Field{{Key: "key", Label: "密钥", Type: "secret"}}}, {ID: "a", InputFields: []Field{{Key: "x", Label: "x", Type: "json"}, {Key: "x", Label: "x", Type: "json"}}}} {
		if e := validateActionInputs(a); e == nil {
			t.Fatal("invalid action schema accepted")
		}
	}
}
