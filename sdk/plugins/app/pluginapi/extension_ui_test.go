package pluginapi

import "testing"

func TestExtensionPageContractRejectsBrokenBindings(t *testing.T) {
	a := Action{ID: "publish", MediaInput: &MediaInput{IDField: "tmdb", TypeField: "kind"}, InputGroups: []InputGroup{{ID: "content", Label: "分享内容"}}, InputFields: []Field{
		{Key: "tmdb", Label: "TMDB", Type: "number"}, {Key: "kind", Label: "类型", Type: "select", Options: []Option{{Value: "movie", Label: "电影"}}},
		{Key: "hashes", Label: "SHA1", Type: "string-list", Group: "content"}, {Key: "quality", Label: "画质", Type: "multi-select", Group: "content", Options: []Option{{Value: "4K", Label: "4K"}}},
	}}
	if err := validateActionInputs(a); err != nil {
		t.Fatal(err)
	}
	a.MediaInput.IDField = "missing"
	if validateActionInputs(a) == nil {
		t.Fatal("unbound picker accepted")
	}
	a.MediaInput.IDField = "tmdb"
	a.InputFields[2].Group = "missing"
	if validateActionInputs(a) == nil {
		t.Fatal("unbound group accepted")
	}
	a.InputFields[2].Group = "content"
	a.InputFields[3].Options = append(a.InputFields[3].Options, Option{Value: "4K", Label: "duplicate"})
	if validateActionInputs(a) == nil {
		t.Fatal("duplicate option accepted")
	}
}
