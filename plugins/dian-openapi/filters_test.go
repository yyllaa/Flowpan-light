package main

import (
	"encoding/json"
	"flowpan/app/pluginapi"
	"strings"
	"testing"
)

func TestResourceFiltersNormalizeBeforeSDKAndIsolateCache(t *testing.T) {
	h := newAPIHost(t)
	r := pluginapi.Request{TaskID: "filters", ActionID: "query", Input: map[string]any{"tmdb_id": 411442, "media_type": "tv", "season": 0, "res": "4K", "chn_sub": "false", "size": 1}}
	result, err := executeOpenAPI(h, r)
	if err != nil {
		t.Fatal(err)
	}
	q := h.requests[0].Input
	if q["chn_sub"] != false || q["season"] != float64(0) || q["page"] != float64(1) || q["sort"] != "hot" {
		t.Fatal(q)
	}
	page := result.Data["pagination"].(map[string]any)
	if page["has_more"] != true {
		t.Fatal(page)
	}
	// Equivalent wire values reuse the same account-scoped result.
	r.Input["chn_sub"] = false
	if _, err = executeOpenAPI(h, r); err != nil || len(h.requests) != 1 {
		t.Fatal(err, h.requests)
	}
	r.Input["page"] = 2
	if _, err = executeOpenAPI(h, r); err != nil || len(h.requests) != 2 {
		t.Fatal(err, h.requests)
	}
	r.Input["chn_sub"] = true
	if _, err = executeOpenAPI(h, r); err != nil || len(h.requests) != 3 {
		t.Fatal(err, h.requests)
	}
	r.Input["refresh"] = true
	if _, err = executeOpenAPI(h, r); err != nil || len(h.requests) != 4 {
		t.Fatal(err, h.requests)
	}
	r.Input["media_type"] = "movie"
	if _, err = executeOpenAPI(h, r); err != nil {
		t.Fatal(err)
	}
	if _, exists := h.requests[4].Input["season"]; exists {
		t.Fatal("movie season sent")
	}
	h.account = "another-account"
	r.Input["refresh"] = false
	if _, err = executeOpenAPI(h, r); err != nil || len(h.requests) != 6 {
		t.Fatal(err, h.requests)
	}
}

func TestInvalidResourceFiltersDoNotCallSDK(t *testing.T) {
	for _, bad := range []map[string]any{{"season": -1}, {"season": 1.5}, {"page": 0}, {"size": 51}, {"tmdb_id": 0}, {"chn_sub": "maybe"}, {"min_size_gb": -1}, {"min_size_gb": 2, "max_size_gb": 1}} {
		h := newAPIHost(t)
		input := map[string]any{"tmdb_id": 123, "media_type": "tv"}
		for key, value := range bad {
			input[key] = value
		}
		if _, err := executeOpenAPI(h, pluginapi.Request{TaskID: "invalid", ActionID: "query", Input: input}); err == nil || len(h.requests) != 0 {
			t.Fatal(bad, err, h.requests)
		}
	}
}

func TestQueryPaginationAndLargeCacheKeepUsableResources(t *testing.T) {
	h := newAPIHost(t)
	// A large raw response fits after keeping just the usable resource page.
	h.listData = map[string]any{"items": []any{map[string]any{"share_id": 25, "resource_id": 123, "file_name": "fixture", "note": strings.Repeat("x", 50000)}}, "has_more": false, "total": 10}
	r := pluginapi.Request{TaskID: "large", ActionID: "query", Input: map[string]any{"tmdb_id": 123, "media_type": "movie", "size": 1}}
	first, err := executeOpenAPI(h, r)
	if err != nil {
		t.Fatal(err)
	}
	if first.Data["pagination"].(map[string]any)["has_more"] != false {
		t.Fatal(first.Data)
	}
	second, err := executeOpenAPI(h, r)
	if err != nil || len(h.requests) != 1 {
		t.Fatal(err, len(h.requests))
	}
	if second.Data["resources"] == nil || second.Data["pagination"] == nil {
		t.Fatal("cache discarded page", second)
	}
	for _, raw := range h.state {
		if len(raw) >= 48<<10 {
			t.Fatal("cache overflow", len(raw))
		}
	}
	// Pages too large for KV are returned intact, never cached as an empty receipt.
	h = newAPIHost(t)
	files := []string{}
	for n := 0; n < 150; n++ {
		files = append(files, strings.Repeat("f", 500))
	}
	h.listData = map[string]any{"items": []any{map[string]any{"share_id": 25, "resource_id": 123, "file_name": "fixture", "file_list": files}}}
	for n := 0; n < 2; n++ {
		result, e := executeOpenAPI(h, r)
		b, _ := json.Marshal(result.Data["resources"])
		if e != nil || len(b) < 44<<10 {
			t.Fatal(e, len(b))
		}
	}
	if len(h.requests) != 2 {
		t.Fatal("oversize page cached without usable resources")
	}
}
