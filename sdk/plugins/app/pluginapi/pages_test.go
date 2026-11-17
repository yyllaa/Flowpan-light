package pluginapi

import "testing"

func TestPluginPageDefinitionAndPublicAssets(t *testing.T) {
	d := Definition{Runtime: "wasm", Pages: []Page{{ID: "music", Label: "音乐刮削", Icon: "file", Entry: "ui/index.html", Navigation: true}, {ID: "settings", Label: "设置"}}}
	if err := d.validatePages(); err != nil {
		t.Fatal(err)
	}
	for _, asset := range []string{"ui/index.html", "ui/assets/app.js", "ui/fonts/font.woff2", "ui/封面.webp"} {
		if !ValidPageAsset(asset) {
			t.Fatal("public asset rejected", asset)
		}
	}
	for _, asset := range []string{"manifest.json", "bin/plugin.wasm", "ui/../config.json", "ui/.env", "ui/a/../../index.html", "ui/main.js.map", "ui/source.go", "ui/a.html?x", "ui/a%2fb.js", "ui/a:b.js", "ui\\a.js", "ui//index.html"} {
		if ValidPageAsset(asset) {
			t.Fatal("unsafe asset accepted", asset)
		}
	}
	for _, bad := range []Page{{ID: "../music", Label: "music"}, {ID: "ok", Label: ""}, {ID: "ok", Label: "a\nb"}, {ID: "ok", Label: "music", Entry: "bin/plugin.wasm"}, {ID: "ok", Label: "music", Entry: "ui/app.js"}, {ID: "ok", Label: "music", Icon: "<svg>"}} {
		if (Definition{Runtime: "wasm", Pages: []Page{bad}}).validatePages() == nil {
			t.Fatal("bad page accepted", bad)
		}
	}
	d.Pages = append(d.Pages, d.Pages[0])
	if d.validatePages() == nil {
		t.Fatal("duplicate page accepted")
	}
	d.Pages = []Page{{ID: "native", Label: "原生工作区"}}
	d.Runtime = "builtin"
	if err := d.validatePages(); err != nil {
		t.Fatal(err)
	}
	d.Pages[0].Entry = "ui/index.html"
	if d.validatePages() == nil {
		t.Fatal("builtin custom frontend accepted")
	}
}
