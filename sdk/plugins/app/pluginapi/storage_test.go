package pluginapi

import "testing"

func TestStorageDefinitionRejectsInvalidProviderContracts(t *testing.T) {
	valid := func() StorageDefinition {
		return StorageDefinition{ID: "samplecloud", Name: "示例网盘", LoginMethods: []string{"token"}, Capabilities: []string{"list", "stat"}, Fields: []Field{{Key: "token", Type: "secret", Label: "Token"}}}
	}
	if err := valid().Validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*StorageDefinition){
		"builtin":              func(d *StorageDefinition) { d.ID = "p139" },
		"unsafe id":            func(d *StorageDefinition) { d.ID = "../cloud" },
		"unknown capability":   func(d *StorageDefinition) { d.Capabilities = append(d.Capabilities, "arbitrary") },
		"duplicate capability": func(d *StorageDefinition) { d.Capabilities = append(d.Capabilities, "list") },
		"missing browse":       func(d *StorageDefinition) { d.Capabilities = []string{"list"} },
		"no login":             func(d *StorageDefinition) { d.LoginMethods = nil },
		"duplicate login":      func(d *StorageDefinition) { d.LoginMethods = []string{"token", "token"} },
		"unknown field":        func(d *StorageDefinition) { d.Fields[0].Type = "filesystem" },
		"empty choices":        func(d *StorageDefinition) { d.Fields[0].Type = "select" },
		"duplicate field":      func(d *StorageDefinition) { d.Fields = append(d.Fields, d.Fields[0]) },
	} {
		t.Run(name, func(t *testing.T) {
			d := valid()
			change(&d)
			if err := d.Validate(); err == nil {
				t.Fatal("invalid contract accepted")
			}
		})
	}
	def := Definition{SchemaVersion: 2, APIVersion: "1", ID: "storage-sample", Name: "示例网盘", Version: "1.0.0", Runtime: "wasm", ConfigVersion: 1, Storage: new(StorageDefinition)}
	*def.Storage = valid()
	if err := def.Validate(); err == nil {
		t.Fatal("storage.provider permission missing")
	}
	def.Permissions = []string{"storage.provider"}
	if err := def.Validate(); err != nil {
		t.Fatal(err)
	}
	def.Storage = nil
	if err := def.Validate(); err == nil {
		t.Fatal("resource plugin accepted no actions")
	}
}
