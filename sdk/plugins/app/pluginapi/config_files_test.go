package pluginapi

import (
	"encoding/json"
	"testing"
)

func TestConfigFilePickerDeclaration(t *testing.T) {
	max := float64(1)
	d := Definition{Fields: []Field{{Key: "source", Type: "account-list", Max: &max}, {Key: "folder", Type: "path", Extensions: Extensions{"flowpan.files": json.RawMessage(`{"account_field":"source"}`)}}}}
	if err := d.validateConfigFileFields(); err != nil {
		t.Fatal(err)
	}
	key, err := d.Fields[1].FilePickerAccount()
	if err != nil || key != "source" {
		t.Fatal(key, err)
	}
	for _, raw := range []string{`{"account_field":"missing"}`, `{"account_field":"folder"}`, `{"account_field":12}`, `null`, `[]`} {
		d.Fields[1].Extensions["flowpan.files"] = json.RawMessage(raw)
		if err := d.validateConfigFileFields(); err == nil {
			t.Fatal("invalid binding accepted", raw)
		}
	}
	d.Fields[1].Extensions = Extensions{"future.unknown": json.RawMessage(`{"custom":true}`)}
	if err := d.validateConfigFileFields(); err != nil {
		t.Fatal("opaque extension rejected", err)
	}
	max = 1.5
	if err := d.validateConfigFileFields(); err == nil {
		t.Fatal("fractional account limit accepted")
	}
}
