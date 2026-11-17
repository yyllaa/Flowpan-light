package pluginapi

import "testing"

func TestPluginAdaptiveRecognitionContracts(t *testing.T) {
	provider := RecognitionProvider{ID: "catalog", Name: "Catalog", Extensions: []string{".mkv", ".strm"}, Patterns: []string{`(?i)\bNX-[0-9]+\b`}}
	if err := (RecognitionDefinition{Providers: []RecognitionProvider{provider}}).Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		want bool
	}{{"NX-001.S02E03.MKV", true}, {"ordinary.mkv", false}, {"NX-001.txt", false}, {"NX-NONE.strm", false}} {
		if got := provider.Matches(tc.name); got != tc.want {
			t.Fatal(tc, got)
		}
	}
	for _, pattern := range []string{"[", ""} {
		p := provider
		p.Patterns = []string{pattern}
		if (RecognitionDefinition{Providers: []RecognitionProvider{p}}).Validate() == nil {
			t.Fatal("invalid pattern", pattern)
		}
	}
	for _, key := range []string{"acme.plugin/catalog", "7-provider/catalog"} {
		if _, _, err := SplitRecognitionProvider(key); err != nil {
			t.Fatal(key, err)
		}
	}
	for _, key := range []string{"../catalog", "x/y/z", "x/", "/catalog"} {
		if _, _, err := SplitRecognitionProvider(key); err == nil {
			t.Fatal(key)
		}
	}
	for _, name := range []string{"recognition.providers", "recognition.identify"} {
		tool, ok := FindTool(name)
		if !ok || tool.BusinessWrite || tool.Permission != "media.read" {
			t.Fatal(tool)
		}
	}
}
func TestPluginAdaptiveRecognitionRouting(t *testing.T) {
	route := &RecognitionRoute{Mode: "native", Rules: []RecognitionRule{{Prefix: "incoming", Mode: "auto", Providers: []string{"a/catalog"}}, {Prefix: "incoming/special", Mode: "provider", Providers: []string{"b/catalog"}}}}
	route.Naming = &RecognitionNaming{File: "{id}"}
	route.Destination = &RecognitionDestination{ID: "selected-root", Path: "/小姐姐"}
	if err := route.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ file, mode, provider string }{{"incoming/a.mkv", "auto", "a/catalog"}, {"incoming/special/a.mkv", "provider", "b/catalog"}, {"incoming2/a.mkv", "native", ""}, {"incoming.mkv", "native", ""}, {"else/incoming/a.mkv", "native", ""}} {
		r := route.Select(tc.file)
		if r.Mode != tc.mode || len(r.Providers) > 0 && r.Providers[0] != tc.provider {
			t.Fatal(tc, r)
		}
	}
	if route.Select("incoming/special/a.mkv").Destination != route.Destination {
		t.Fatal("destination not inherited")
	}
	route.Rules[1].Destination = &RecognitionDestination{ID: "other-root"}
	if route.Select("incoming/special/a.mkv").Destination.ID != "other-root" || route.Select("incoming/a.mkv").Destination.ID != "selected-root" {
		t.Fatal("rule destination leaked")
	}
	if (&RecognitionRoute{Destination: &RecognitionDestination{ID: ""}}).Validate() == nil {
		t.Fatal("empty destination accepted")
	}
	if route.Select("incoming/special/a.mkv").Naming != route.Naming {
		t.Fatal("task naming not inherited")
	}
	for _, prefix := range []string{"/absolute", "../escape", "a/../b", "a//b", "C:/escape", `a\b`, "."} {
		r := &RecognitionRoute{Mode: "native", Rules: []RecognitionRule{{Prefix: prefix, Mode: "native"}}}
		if r.Validate() == nil {
			t.Fatal("accepted prefix", prefix)
		}
	}
	if (&RecognitionRoute{Mode: "provider"}).Validate() == nil {
		t.Fatal("empty provider accepted")
	}
	if (&RecognitionRoute{Mode: "native", Naming: &RecognitionNaming{Directories: []string{"../other"}}}).Validate() == nil {
		t.Fatal("traversal accepted")
	}
	zero := 0
	r := RecognitionResult{Status: "matched", Metadata: &RecognitionMetadata{ID: "NX-001", Title: "Fixture", Type: "movie", Season: &zero, Episode: &zero}}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	r.Metadata.ID = ""
	if r.Validate() == nil {
		t.Fatal("empty independent ID accepted")
	}
	for _, state := range []string{"no_match", "not_found", "ambiguous", "error"} {
		if err := (RecognitionResult{Status: state}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPluginAdaptiveRecognitionCategoryDeclaration(t *testing.T) {
	definition := RecognitionDefinition{Categories: []RecognitionCategory{{ID: "special", Name: "专用分类"}}, Providers: []RecognitionProvider{{ID: "catalog", Name: "Catalog", Keywords: []string{"Sample-Key"}, Category: "special", Extensions: []string{".mkv"}}}}
	if err := definition.Validate(); err != nil {
		t.Fatal(err)
	}
	p := definition.Providers[0]
	if !p.Matches("prefix.SAMPLE-KEY.mkv") || p.Matches("sample-key.txt") || p.Matches("ordinary.mkv") {
		t.Fatal("keyword or extension claim")
	}
	for _, word := range []string{"", " ", "x", "bad\nword"} {
		copy := definition
		copy.Providers = []RecognitionProvider{p}
		copy.Providers[0].Keywords = []string{word}
		if copy.Validate() == nil {
			t.Fatal("invalid keyword", word)
		}
	}
	for _, key := range []string{"missing", "../escape"} {
		copy := definition
		copy.Providers = []RecognitionProvider{p}
		copy.Providers[0].Category = key
		if copy.Validate() == nil {
			t.Fatal("unbound category", key)
		}
	}
	duplicate := definition
	duplicate.Categories = append(duplicate.Categories, duplicate.Categories[0])
	if duplicate.Validate() == nil {
		t.Fatal("duplicate category ID")
	}
}
