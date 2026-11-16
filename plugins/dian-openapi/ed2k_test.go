package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"flowpan/app/pluginapi"
)

type ed2kHost struct {
	*apiFixtureHost
	task   pluginapi.ED2KTask
	denied bool
}

func (h *ed2kHost) Call(method string, input any, output any) error {
	if method == "ed2k.task" || method == "ed2k.list" {
		if h.denied {
			return errors.New("directory revoked")
		}
		var value any = h.task
		if method == "ed2k.list" {
			value = pluginapi.ED2KPage{Items: []pluginapi.ED2KTask{h.task}, Total: 1, Limit: 20}
		}
		b, _ := json.Marshal(value)
		return json.Unmarshal(b, output)
	}
	return h.apiFixtureHost.Call(method, input, output)
}
func ed2kFixture(t *testing.T) *ed2kHost {
	return &ed2kHost{apiFixtureHost: newAPIHost(t), task: pluginapi.ED2KTask{ID: strings.Repeat("a", 40), Status: "success", Name: "测试.mkv", Path: "/资源/测试.mkv", Size: 1024, Link: "ed2k://|file|测试.mkv|1024|" + strings.Repeat("B", 32) + "|/"}}
}
func TestED2KSelectionPublishesUsingValidatedLibraryResult(t *testing.T) {
	h := ed2kFixture(t)
	r := apiRequest("share")
	r.Input = publishFixture()
	r.Input["share_kind"] = "offline"
	delete(r.Input, "share_url")
	delete(r.Input, "sha1s")
	r.Input["ed2k_ids"] = []string{h.task.ID}
	result, err := executeOpenAPI(h, r)
	if err != nil || result.Failed {
		t.Fatal(result, err)
	}
	if len(h.requests) != 1 || h.requests[0].Input["offline_url"] != h.task.Link || h.requests[0].Input["ed2k_ids"] != nil {
		t.Fatal("selection was not resolved before publication", h.requests)
	}
}
func TestED2KSelectionRejectsUnavailableOrRevokedRecordBeforePublishing(t *testing.T) {
	for _, kind := range []string{"revoked", "unfinished", "invalid-id", "mixed-links"} {
		t.Run(kind, func(t *testing.T) {
			h := ed2kFixture(t)
			r := apiRequest("share")
			r.Input = publishFixture()
			r.Input["share_kind"] = "offline"
			r.Input["ed2k_ids"] = []string{h.task.ID}
			switch kind {
			case "revoked":
				h.denied = true
			case "unfinished":
				h.task.Status = "running"
			case "invalid-id":
				r.Input["ed2k_ids"] = []string{"not-an-id"}
			case "mixed-links":
				r.Input["offline_url"] = "magnet:?xt=urn:btih:" + strings.Repeat("a", 40)
			}
			if _, err := executeOpenAPI(h, r); err == nil {
				t.Fatal("invalid selection accepted")
			}
			if len(h.requests) != 0 {
				t.Fatal("publication issued before validation")
			}
		})
	}
}
func TestED2KOptionSourceDoesNotPublishOrCallExternalSDK(t *testing.T) {
	h := ed2kFixture(t)
	r := apiRequest("ed2k-library")
	r.Input = map[string]any{"query": "测试", "limit": 20}
	result, err := executeOpenAPI(h, r)
	if err != nil || len(result.Data["options"].([]pluginapi.Option)) != 1 || len(h.requests) != 0 {
		t.Fatal(result, err)
	}
}
