package main

import (
	"encoding/json"
	"flowpan/app/pluginapi"
	"strings"
	"testing"
)

func publishFixture() map[string]any {
	return map[string]any{"tmdb_id": 411442, "media_type": "movie", "share_kind": "115", "share_url": "https://115.com/s/fixture?password=test", "sha1s": []string{strings.Repeat("a", 40)}, "resolution": "1080P", "file_extension": "MKV", "subtitle_kind": "embedded_chs"}
}

func TestTypedPublicationNormalizesBeforeSDKAndReusesReceipt(t *testing.T) {
	h := newAPIHost(t)
	req := apiRequest("share")
	req.Input = publishFixture()
	req.Input["source_tag"] = "WEB-DL"
	req.Input["codec"] = "HEVC"
	req.Input["sha1s"] = strings.Repeat("a", 40) + "\n" + strings.Repeat("A", 40)
	result, err := executeOpenAPI(h, req)
	if err != nil || result.Data["share_status"] != "已发布" {
		t.Fatal(err, result)
	}
	input := h.requests[0].Input
	if input["tag"] != "DT2|1080P|WEB-DL|HEVC|CHS|MUXED-SUB|MKV" || input["resolution"] != nil || input["codec"] != nil {
		t.Fatal(input)
	}
	hashes := input["sha1s"].([]any)
	if len(hashes) != 1 || hashes[0] != strings.Repeat("A", 40) {
		t.Fatal(hashes)
	}
	if _, err := executeOpenAPI(h, req); err != nil || len(h.operations) != 1 {
		t.Fatal("publication replay", err, h.operations)
	}
}

func TestInvalidPublicationNeverCallsSDK(t *testing.T) {
	cases := map[string]func(map[string]any){
		"work":          func(m map[string]any) { delete(m, "tmdb_id") },
		"sha1":          func(m map[string]any) { m["sha1s"] = "invalid" },
		"empty-hashes":  func(m map[string]any) { m["sha1s"] = []string{} },
		"link":          func(m map[string]any) { m["share_url"] = "https://115.com.evil.test/s/id" },
		"type":          func(m map[string]any) { m["media_type"] = "other" },
		"format":        func(m map[string]any) { delete(m, "file_extension") },
		"resolution":    func(m map[string]any) { delete(m, "resolution") },
		"negative-cost": func(m map[string]any) { m["unlock_cost_override"] = -1 },
		"too-many-files": func(m map[string]any) {
			files := []string{}
			for i := 0; i < 501; i++ {
				files = append(files, strings.Repeat("a", i+1))
			}
			m["file_list"] = files
		},
		"mixed-offline": func(m map[string]any) {
			m["share_kind"] = "offline"
			m["offline_url"] = "magnet:?xt=urn:btih:" + strings.Repeat("a", 40) + "\ned2k://|file|test.mkv|123|" + strings.Repeat("b", 32) + "|/"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			h := newAPIHost(t)
			req := apiRequest("share")
			req.Input = publishFixture()
			mutate(req.Input)
			if _, err := executeOpenAPI(h, req); err == nil || len(h.operations) != 0 || len(h.state) != 0 {
				t.Fatal("invalid input reached SDK or receipt", err, h.operations)
			}
		})
	}
}

func TestTVSpecialAndOfflinePublication(t *testing.T) {
	m := publishFixture()
	m["media_type"] = "tv"
	m["season"] = 0
	m["share_kind"] = "offline"
	m["offline_url"] = "magnet:?xt=urn:btih:" + strings.Repeat("a", 40)
	if err := normalizeShare(m); err != nil {
		t.Fatal(err)
	}
	if m["season"] != float64(0) || m["seasons_csv"] != "0" || m["offline_type"] != "magnet" || m["share_url"] != nil {
		t.Fatal(m)
	}
	m["seasons_csv"] = "1"
	if normalizeShare(m) == nil {
		t.Fatal("special mixed with regular seasons")
	}
}

func TestHardcodedSubtitleUsesDocumentedDT2Token(t *testing.T) {
	m := publishFixture()
	m["subtitle_kind"] = "hardcoded_chs"
	if err := normalizeShare(m); err != nil {
		t.Fatal(err)
	}
	if m["tag"] != "DT2|1080P|CHS|HARDSUB|MKV" {
		t.Fatal(m["tag"])
	}
}

func TestTypedRSSAndShareLinkLookup(t *testing.T) {
	h := newAPIHost(t)
	req := apiRequest("rss")
	req.Input = map[string]any{"media_types": []string{"movie"}, "categories": "10\n20\n10", "regions": []string{}, "resolutions": []string{"4K"}, "limit": 25}
	result, err := executeOpenAPI(h, req)
	if err != nil || result.Table == nil || result.Data["item_count"] != 0 {
		t.Fatal(err, result)
	}
	sent := h.requests[0].Input
	filter := sent["filter"].(map[string]any)
	if len(filter["categories"].([]any)) != 2 || filter["tags"].(map[string]any)["resolution"].([]any)[0] != "4K" || sent["categories"] != nil {
		t.Fatal(sent)
	}
	req = apiRequest("share-check")
	req.Input = map[string]any{"share_code": "https://115.com/s/fixture?password=test"}
	result, err = executeOpenAPI(h, req)
	if err != nil || result.Message != "这条分享已发布" || h.requests[1].Input["share_code"] != "fixture" {
		t.Fatal(err, result)
	}
}

func TestExtensionResultsAreReadableTextAndTables(t *testing.T) {
	var response map[string]any
	json.Unmarshal([]byte(`{"items":[{"file_name":"movie.mkv","file_size":1073741824,"sha1":"ABC","status":"ready","path":"/cache/movie.mkv"}],"total":3}`), &response)
	result := pluginapi.Result{Data: map[string]any{}}
	extensionResult(&result, "virtual-cache", response, map[string]any{"page": float64(1), "size": float64(1)})
	if result.Table == nil || result.Table.Rows[0]["size"] != "1.00 GiB" || result.Table.Rows[0]["name"] != "movie.mkv" || result.Data["pagination"].(map[string]any)["has_more"] != true {
		t.Fatal(result)
	}
	result = pluginapi.Result{Data: map[string]any{}}
	extensionResult(&result, "profile", map[string]any{"nickname": "name", "points": float64(18), "vip_active": true, "entitlements": map[string]any{"daily_api_limit": float64(500)}}, nil)
	if result.Data["points"] != float64(18) || result.Data["membership"] != "有效" || len(result.Items) != 2 {
		t.Fatal(result)
	}
}

func TestTypedDeveloperRangeValidationBeforeSDK(t *testing.T) {
	h := newAPIHost(t)
	req := apiRequest("developer-usage")
	req.Input = map[string]any{"since": "2026-11-12T00:00:00+08:00", "until": "2026-11-01T00:00:00+08:00"}
	if _, err := executeOpenAPI(h, req); err == nil || len(h.operations) != 0 {
		t.Fatal("reversed range reached SDK", err)
	}
}

func TestMissingAvatarIsAnEmptyStateWithoutInvalidSDKCall(t *testing.T) {
	h := newAPIHost(t)
	h.meData = map[string]any{"avatar_url": ""}
	req := apiRequest("avatar")
	req.Input = map[string]any{}
	result, err := executeOpenAPI(h, req)
	if err != nil || result.Message != "账号未设置头像" || len(h.operations) != 1 || h.operations[0] != "me" {
		t.Fatal(err, result, h.operations)
	}
	if _, err := executeOpenAPI(h, req); err != nil || len(h.operations) != 1 {
		t.Fatal("empty avatar repeated API", err, h.operations)
	}
}

func TestProfileDisplaysNestedRealEntitlements(t *testing.T) {
	result := pluginapi.Result{Data: map[string]any{}}
	response := map[string]any{"points": float64(30), "is_vip": false, "entitlements": map[string]any{"membership": "normal", "remaining": map[string]any{"api_daily": float64(120), "unlock_daily": float64(0)}, "growth": map[string]any{"level": float64(2), "name": "level2"}}}
	extensionResult(&result, "profile", response, nil)
	if result.Data["membership"] != "普通会员" || result.Data["quota"] != "120" || len(result.Items) != 4 {
		t.Fatal(result)
	}
}
