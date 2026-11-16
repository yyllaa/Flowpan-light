package main

import (
	"encoding/json"
	"flowpan/app/pluginapi"
	"fmt"
	"testing"
)

func TestPublicationTMDBIDUsesFullDecimalInResultsAndReceipts(t *testing.T) {
	cases := []struct {
		name, mediaType, decimal string
		id                       any
	}{
		{"screenshot-movie", "movie", "1257985", float64(1257985)},
		{"screenshot-tv", "tv", "1257985", float64(1.257985e6)},
		{"six-digits", "movie", "411442", 411442},
		{"below-exponent", "movie", "999999", float64(999999)},
		{"exponent-boundary", "tv", "1000000", 1000000},
		{"largest-accepted", "movie", "1000000000000", json.Number("1000000000000")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newAPIHost(t)
			req := apiRequest("share")
			req.Input = publishFixture()
			req.Input["tmdb_id"] = c.id
			req.Input["media_type"] = c.mediaType
			result, err := executeOpenAPI(h, req)
			if err != nil {
				t.Fatal(err)
			}
			want := "TMDB " + c.decimal + " · " + map[string]string{"movie": "电影", "tv": "剧集"}[c.mediaType]
			check := func(stage string, result pluginapi.Result) {
				t.Helper()
				for _, item := range result.Items {
					if item.Label == "作品" {
						t.Logf("%s displayed=%q", stage, item.Message)
						if item.Message != want {
							t.Errorf("%s displayed=%q, want %q", stage, item.Message, want)
						}
						return
					}
				}
				t.Errorf("%s missing work label", stage)
			}
			check("first", result)
			if len(h.requests) != 1 || h.requests[0].Operation != "share" {
				t.Fatal("unexpected publish operations", h.operations)
			}
			payload, _ := json.Marshal(h.requests[0].Input["tmdb_id"])
			t.Logf("submitted_tmdb_id=%s", payload)
			if string(payload) != c.decimal {
				t.Errorf("submitted TMDB ID=%s, want %s", payload, c.decimal)
			}
			// Existing stored receipts must be rendered again without republishing.
			for key, raw := range h.state {
				var cached receipt
				if err := json.Unmarshal(raw, &cached); err != nil {
					t.Fatal(err)
				}
				cached.Result.Items = []pluginapi.ResultItem{{Label: "作品", Message: fmt.Sprintf("TMDB %v · 旧回执", c.id)}}
				h.state[key], _ = json.Marshal(cached)
			}
			replayed, err := executeOpenAPI(h, req)
			if err != nil {
				t.Fatal(err)
			}
			check("cached", replayed)
			if len(h.operations) != 1 {
				t.Fatal("cached display republished share", h.operations)
			}
			t.Log("publish_requests=1 cached_publish_requests=0")
		})
	}
}
