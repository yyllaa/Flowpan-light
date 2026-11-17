package pluginapi

import (
	"encoding/json"
	"testing"
)

func TestStorageUIContract(t *testing.T) {
	var d StorageDefinition
	if err := json.Unmarshal([]byte(`{"id":"futurecloud","name":"Future cloud","login_methods":["qrcode"],"capabilities":["list","stat"],"fields":[{"key":"advanced","label":"Advanced","type":"boolean","default":false},{"key":"client_version","label":"Version","type":"select","options_source":"versions","depends_on":["advanced"],"visible_when":{"field":"advanced","equals":true}},{"key":"region","label":"Region","type":"text","read_only":true,"default":"cn"}],"task_fields":{"strm":[{"key":"quality","label":"Quality","type":"select","default":"original","options":[{"value":"original","label":"Original"}]}],"upload":[{"key":"parallel","label":"Parallel","type":"number","default":0}]}}`), &d); err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("storage UI contract rejected: %v", err)
	}
	t.Log("STORAGE_UI_CONTRACT_OK qr_extra_fields=accepted conditional=accepted readonly=accepted dynamic=accepted tasks=accepted")
}
