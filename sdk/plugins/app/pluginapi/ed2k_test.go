package pluginapi

import "testing"

func TestPluginAdaptiveED2KAndDynamicSourceContract(t *testing.T) {
	d := Definition{SchemaVersion: SchemaVersion, APIVersion: APIVersion, ID: "library-fixture", Version: "1.0.0", Name: "结果库", Runtime: "wasm", ConfigVersion: 1, Permissions: []string{"ed2k.read", "ed2k.generate"}, Actions: []Action{{ID: "list", Label: "查询", Kind: "probe", Placement: "context"}, {ID: "generate", Label: "生成", Kind: "run", Placement: "context", InputFields: []Field{{Key: "selection", Label: "选择记录", Type: "multi-select", OptionsSource: "list"}}}}, Contributions: []Contribution{{Point: "file-selection", Action: "generate", Label: "生成", FileTypes: []string{"file"}, SourceTypes: []string{"future-provider"}}}}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ed2k.submit", "ed2k.list", "ed2k.task", "ed2k.retry", "ed2k.cancel", "ed2k.remove"} {
		tool, ok := FindTool(name)
		if !ok || !KnownPermission(tool.Permission) {
			t.Fatal("tool not registered", name)
		}
	}
	d.Actions[0].Kind = "run"
	if d.Validate() == nil {
		t.Fatal("write action accepted as an option source")
	}
	d.Actions[0].Kind = "probe"
	d.Actions[1].InputFields[0].OptionsSource = "missing"
	if d.Validate() == nil {
		t.Fatal("missing option source accepted")
	}
	d.Actions[1].InputFields[0].OptionsSource = "list"
	d.Contributions[0].SourceTypes = []string{"<script>"}
	if d.Validate() == nil {
		t.Fatal("invalid provider identifier accepted")
	}
}
