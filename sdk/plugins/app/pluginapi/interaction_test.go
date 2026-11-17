package pluginapi

import (
	"encoding/json"
	"testing"
)

func TestPluginAdaptiveInteractionManifestValidation(t *testing.T) {
	base := Definition{SchemaVersion: 2, APIVersion: "1", ID: "future-plugin", Version: "1.0.0", Name: "Future", ConfigVersion: 1, Runtime: "wasm", Permissions: []string{"interaction.receive", "interaction.read"}, Actions: []Action{{ID: "start", Label: "Start", Kind: "run"}}, Interaction: &InteractionDefinition{Commands: []InteractionCommand{{Name: "future", Description: "Open future tool", Action: "start"}}, MessageAction: "start"}}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Definition){
		func(d *Definition) { d.Permissions = nil },
		func(d *Definition) { d.Interaction.Commands[0].Name = "status" },
		func(d *Definition) { d.Interaction.Commands[0].Name = "bad-command" },
		func(d *Definition) { d.Interaction.Commands[0].Action = "missing" },
		func(d *Definition) { d.Interaction.MessageAction = "missing" },
		func(d *Definition) { d.Actions[0].Kind = "probe" },
		func(d *Definition) {
			d.Interaction.Commands = append(d.Interaction.Commands, d.Interaction.Commands[0])
		},
	} {
		b, _ := json.Marshal(base)
		var d Definition
		_ = json.Unmarshal(b, &d)
		change(&d)
		if d.Validate() == nil {
			t.Fatal("invalid interaction accepted", d)
		}
	}
	legacy := base
	legacy.Interaction = nil
	legacy.Permissions = nil
	if err := legacy.Validate(); err != nil {
		t.Fatal("legacy definition changed", err)
	}
	if _, ok := FindTool("interaction.reply"); !ok || !KnownPermission("interaction.receive") {
		t.Fatal("missing registry")
	}
}
