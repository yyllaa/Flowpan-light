package main

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
)

func structRequest() pluginwasm.HTTPRequest {
	return pluginwasm.HTTPRequest{URL: "https://itunes.apple.com/search?term=test"}
}
func TestSearchDecodersAndIDs(t *testing.T) {
	fixtures := map[string]string{
		"netease":     `{"code":200,"result":{"songs":[{"id":2579851234,"name":"晴天","ar":[{"name":"周杰伦"}],"al":{"name":"叶惠美","picUrl":"https://p1.music.126.net/a.jpg"},"dt":269000,"no":3}]}}`,
		"qq":          `{"code":0,"req_0":{"code":0,"data":{"body":{"song":{"list":[{"mid":"abc123","title":"晴天","singer":[{"name":"周杰伦"}],"album":{"mid":"album1","name":"叶惠美"},"interval":269,"index_album":3}]}}}}}`,
		"itunes":      `{"resultCount":1,"results":[{"trackId":2579851234,"trackName":"晴天","artistName":"周杰伦","collectionName":"叶惠美","trackTimeMillis":269000}]}`,
		"musicbrainz": `{"recordings":[{"id":"mbid-1","title":"晴天","length":269000,"artist-credit":[{"name":"周杰伦"}],"releases":[{"title":"叶惠美","date":"2003-07-31"}]}]}`,
	}
	for source, raw := range fixtures {
		t.Run(source, func(t *testing.T) {
			items, err := parseSearch(source, raw)
			if err != nil || len(items) != 1 {
				t.Fatalf("parse %v %v", items, err)
			}
			if items[0].Tags.Title != "晴天" || items[0].Tags.Artist != "周杰伦" || strings.Contains(items[0].ID, "e+") {
				t.Fatalf("lost values %+v", items[0])
			}
		})
	}
	for _, source := range []string{"netease", "qq", "itunes", "musicbrainz"} {
		if _, err := parseSearch(source, `{"error":"blocked"}`); err == nil {
			t.Fatalf("accepted malformed %s", source)
		}
	}
}
func TestFilenameHintsDoNotHideMissingTags(t *testing.T) {
	input, err := fromProbe(json.RawMessage(`{"format":{"duration":"180","tags":{}},"streams":[{"codec_type":"audio"}]}`), "周杰伦 - 晴天.flac", "@2 - @1")
	if err != nil || input.Title != "晴天" || input.Artist != "周杰伦" || input.Native.Title != "" {
		t.Fatalf("bad filename extraction %+v %v", input, err)
	}
	chosen := candidate{Tags: tags{MediaMetadata: pluginapi.MediaMetadata{Title: "晴天", Artist: "周杰伦", Album: "叶惠美"}}}
	if desired(input, chosen, "missing").Title != "晴天" {
		t.Fatal("missing tag not written")
	}
}
func TestMatchAmbiguityVersionAndSimplifiedChinese(t *testing.T) {
	original := tags{MediaMetadata: pluginapi.MediaMetadata{Title: "晴天", Artist: "周杰倫"}, Duration: 269}
	choices := []candidate{{Source: "qq", ID: "1", Tags: tags{MediaMetadata: pluginapi.MediaMetadata{Title: "晴天", Artist: "周杰伦", Album: "叶惠美"}, Duration: 269}}}
	if _, ok := rank(original, choices); !ok {
		t.Fatal("simplified/traditional match rejected")
	}
	choices = append(choices, candidate{Source: "qq", ID: "2", Tags: tags{MediaMetadata: pluginapi.MediaMetadata{Title: "晴天", Artist: "周杰伦", Album: "Live"}, Duration: 269}})
	if _, ok := rank(original, choices); ok {
		t.Fatal("ambiguous editions auto accepted")
	}
	choices = choices[:1]
	choices[0].Tags.Duration = 320
	if _, ok := rank(original, choices); ok {
		t.Fatal("duration mismatch auto accepted")
	}
}
func TestWeapiProducesFormWithoutNativeDLL(t *testing.T) {
	for n := 0; n < 3; n++ {
		form, err := weapi([]byte(`{"s":"晴天 周杰伦","type":1,"limit":15}`))
		if err != nil {
			t.Fatal(err)
		}
		v, err := url.ParseQuery(form)
		if err != nil || len(v.Get("encSecKey")) != 256 || v.Get("params") == "" {
			t.Fatal("invalid weapi form")
		}
	}
}
