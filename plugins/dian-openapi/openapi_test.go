package main

import (
	"encoding/json"
	"flowpan/app/pluginapi"
	"fmt"
	"strings"
	"testing"
)

type apiFixtureHost struct {
	t          *testing.T
	account    string
	state      map[string]json.RawMessage
	operations []string
	requests   []pluginapi.SDKRequest
	listData   map[string]any
	meData     map[string]any
	failure    *pluginapi.SDKFailure
}

func newAPIHost(t *testing.T) *apiFixtureHost {
	return &apiFixtureHost{t: t, account: "fixture-account", state: map[string]json.RawMessage{}}
}
func (h *apiFixtureHost) Call(method string, p any, out any) error {
	b, _ := json.Marshal(p)
	var args map[string]json.RawMessage
	_ = json.Unmarshal(b, &args)
	var result any
	switch method {
	case "sdk.call":
		var q pluginapi.SDKRequest
		_ = json.Unmarshal(b, &q)
		if q.Provider != "dian115" {
			h.t.Fatal("wrong host SDK")
		}
		reply := pluginapi.SDKResponse{Account: h.account}
		if q.Operation != "identity" {
			h.operations = append(h.operations, q.Operation)
			h.requests = append(h.requests, q)
			if h.failure != nil {
				reply.Failure = h.failure
			} else {
				data := map[string]any{"code": "ok"}
				switch q.Operation {
				case "me":
					data = map[string]any{"id": 7, "nickname": "fixture", "points": 105, "avatar_url": "/api/portal/avatars/fixture.png"}
					if h.meData != nil {
						data = h.meData
					}
				case "list-shares":
					data = map[string]any{"items": []any{map[string]any{"share_id": 25, "resource_id": 123, "file_name": "fixture.mkv", "unlock_cost": 5, "share_kind": "115", "is_unlocked": true}}}
					if h.listData != nil {
						data = h.listData
					}
				case "signin":
					data["new_balance"] = 105
				case "unlock":
					data["payload"] = map[string]any{"share_kind": "115", "share_code": "fixture-code", "receive_code": "test"}
				case "avatar":
					data["content_type"] = "image/png"
					data["data_base64"] = "iVBORw0KGgo="
				case "rss":
					data["items"] = []any{}
					data["next_cursor"] = "next"
					data["has_more"] = false
				case "check-sharecode":
					data["exists"] = true
				}
				reply.Data, _ = json.Marshal(data)
			}
		}
		result = reply
	case "kv.get":
		var key string
		_ = json.Unmarshal(args["key"], &key)
		if raw, ok := h.state[key]; ok {
			return json.Unmarshal(raw, out)
		}
		result = receipt{}
	case "kv.set":
		var key string
		_ = json.Unmarshal(args["key"], &key)
		h.state[key] = append([]byte{}, args["value"]...)
		return nil
	case "task.progress", "notify":
		return nil
	default:
		h.t.Fatalf("public plugin attempted privileged/direct API tool %s", method)
	}
	if out == nil {
		return nil
	}
	encoded, _ := json.Marshal(result)
	return json.Unmarshal(encoded, out)
}
func apiRequest(action string) pluginapi.Request {
	return pluginapi.Request{ActionID: action, TaskID: "fixture-" + action, Input: map[string]any{"tmdb_id": float64(411442), "media_type": "movie", "resource_id": "25:123"}, Confirmed: true}
}
func TestOpenAPIAllActionsAndPersistentReceipts(t *testing.T) {
	h := newAPIHost(t)
	for _, action := range []string{"status", "search", "checkin", "unlock"} {
		r, e := executeOpenAPI(h, apiRequest(action))
		if e != nil || r.Failed {
			t.Fatal(action, e, r.Message)
		}
		if action != "status" {
			n := len(h.operations)
			_, e = executeOpenAPI(h, apiRequest(action))
			if e != nil || len(h.operations) != n {
				t.Fatal("repeat used API", action, e)
			}
		}
	}
	if len(h.operations) != 4 {
		t.Fatal(h.operations)
	}
}
func TestAllExtendedOperationsUseHostSDK(t *testing.T) {
	cases := []struct {
		action, operation string
		input             map[string]any
	}{
		{"profile", "me", map[string]any{}},
		{"avatar", "avatar", map[string]any{"avatar_url": "/api/portal/avatars/fixture.png"}},
		{"query", "list-shares", map[string]any{"tmdb_id": 1, "media_type": "movie", "filters": map[string]any{"res": "4K"}}},
		{"virtual-cache", "virtual-cache", map[string]any{"tmdb_id": 1, "media_type": "movie"}},
		{"sha1-query", "tmdb-sha1", map[string]any{"tmdb_id": 1, "media_type": "movie", "sha1s": []string{strings.Repeat("a", 40)}}},
		{"share-check", "check-sharecode", map[string]any{"share_code": "fixture"}},
		{"rss-options", "rss-options", map[string]any{}},
		{"rss", "rss", map[string]any{"filter": map[string]any{}, "limit": 50}},
		{"share", "share", map[string]any{"payload": publishFixture()}},
		{"developer-usage", "developer-usage", map[string]any{"payload": map[string]any{}}},
		{"signin-lucky", "signin", map[string]any{}},
	}
	for _, c := range cases {
		t.Run(c.action, func(t *testing.T) {
			h := newAPIHost(t)
			r := apiRequest(c.action)
			r.Input = c.input
			result, e := executeOpenAPI(h, r)
			if e != nil || result.Failed {
				t.Fatal(e, result)
			}
			if len(h.operations) != 1 || h.operations[0] != c.operation {
				t.Fatal(h.operations)
			}
			before := len(h.operations)
			_, e = executeOpenAPI(h, r)
			if e != nil || len(h.operations) != before {
				t.Fatal("repeat API", e, h.operations)
			}
		})
	}
}
func TestOpenAPIConfirmationAndAmbiguousWriteProtection(t *testing.T) {
	for _, action := range []string{"unlock", "checkin", "share", "signin-lucky"} {
		t.Run(action, func(t *testing.T) {
			h := newAPIHost(t)
			r := apiRequest(action)
			if action == "share" {
				r.Input = map[string]any{"payload": publishFixture()}
			}
			if action != "checkin" {
				r.Confirmed = false
				if _, e := executeOpenAPI(h, r); e == nil || len(h.operations) != 0 {
					t.Fatal("missing confirmation")
				}
				r.Confirmed = true
			}
			h.failure = &pluginapi.SDKFailure{Message: "timeout"}
			if _, e := executeOpenAPI(h, r); e == nil {
				t.Fatal("timeout accepted")
			}
			n := len(h.operations)
			if n != 1 {
				t.Fatal("write did not reach SDK", h.operations)
			}
			h.failure = nil
			if _, e := executeOpenAPI(h, r); e == nil || len(h.operations) != n {
				t.Fatal("ambiguous write replayed", e)
			}
		})
	}
}
func TestAuthenticatedRejectionCanBeRetriedAndKeysAreIsolated(t *testing.T) {
	h := newAPIHost(t)
	r := apiRequest("unlock")
	h.failure = &pluginapi.SDKFailure{Status: 429, Code: "rate_limited", Message: "limit", Verified: true}
	if _, e := executeOpenAPI(h, r); e == nil {
		t.Fatal("quota accepted")
	}
	h.failure = nil
	if _, e := executeOpenAPI(h, r); e != nil {
		t.Fatal("verified rejection retained pending", e)
	}
	n := len(h.operations)
	h.account = "other-account"
	if _, e := executeOpenAPI(h, r); e != nil || len(h.operations) != n+1 {
		t.Fatal("account cache leaked", e)
	}
}
func TestDeveloperUsageQuotaAndInputValidation(t *testing.T) {
	h := newAPIHost(t)
	r := apiRequest("developer-usage")
	r.Input = map[string]any{"payload": map[string]any{"page": 1}}
	if _, e := executeOpenAPI(h, r); e != nil {
		t.Fatal(e)
	}
	r.Input["payload"] = map[string]any{"page": 2}
	if _, e := executeOpenAPI(h, r); e == nil || len(h.operations) != 1 {
		t.Fatal("12h quota ignored")
	}
	for _, bad := range []any{[]any{}, "null", "[]", "not-json"} {
		if _, e := objectInput(bad); e == nil {
			t.Fatal(fmt.Sprint("invalid object accepted ", bad))
		}
	}
}

func TestDefaultAvatarReusesMetadataAndImageCache(t *testing.T) {
	h := newAPIHost(t)
	r := apiRequest("avatar")
	r.Input = map[string]any{}
	if _, err := executeOpenAPI(h, r); err != nil {
		t.Fatal(err)
	}
	if len(h.operations) != 2 || h.operations[0] != "me" || h.operations[1] != "avatar" {
		t.Fatal(h.operations)
	}
	if _, err := executeOpenAPI(h, r); err != nil || len(h.operations) != 2 {
		t.Fatal("avatar cache repeated metadata request", err, h.operations)
	}
}
