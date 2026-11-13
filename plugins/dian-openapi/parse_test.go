package main

import (
	"encoding/json"
	"flowpan/app/pluginapi"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDianParsersKeepShareAndOfflineLinks(t *testing.T) {
	cases := []struct{ raw, want string }{
		{`{"payload":{"share_kind":"115","share_code":"abc","receive_code":"xy12"},"new_balance":12}`, "https://115cdn.com/s/abc?password=xy12"},
		{`{"payload":{"share_kind":"offline","urls":["magnet:?xt=urn:btih:fixture"]}}`, "magnet:?xt=urn:btih:fixture"},
		{`{"already":true,"payload":{"url_115":"https://115.com/s/abc?password=xy12"}}`, "https://115.com/s/abc?password=xy12"},
	}
	for _, c := range cases {
		v, e := parseDianUnlock([]byte(c.raw))
		if e != nil {
			t.Fatal(e)
		}
		if v.Share != c.want && (len(v.Links) != 1 || v.Links[0] != c.want) {
			t.Fatal(v)
		}
	}
	if _, e := parseDianUnlock([]byte(`{"payload":{}}`)); e == nil {
		t.Fatal("empty unlock accepted")
	}
	if !dianAlreadySigned(&pluginapi.SDKFailure{Code: "already_signed", Message: "今日已签到", Verified: true}) {
		t.Fatal("already signed not recognized")
	}
}

func TestLargeResourceListsFitHostProtocol(t *testing.T) {
	shares := make([]dianShare, 250)
	for n := range shares {
		shares[n] = dianShare{ID: n + 1, TitleOverride: strings.Repeat("<", 2000), TotalSizeHuman: strings.Repeat("大", 500), FileList: []string{strings.Repeat("<", 1000), strings.Repeat("文件", 1000)}}
	}
	data := resourceResult(shares)
	b, e := json.Marshal(data)
	if e != nil || len(b) > 768<<10 {
		t.Fatal("oversized protocol", len(b), e)
	}
	if len(data["resources"].([]resource)) == 0 {
		t.Fatal("lost all resources")
	}
}

type cachedTMDBHost struct {
	t     *testing.T
	id    string
	calls int
}

func (h *cachedTMDBHost) Call(method string, params any, out any) error {
	h.calls++
	if method == "sdk.call" {
		b, _ := json.Marshal(pluginapi.SDKResponse{Account: "numeric"})
		return json.Unmarshal(b, out)
	}
	if method != "kv.get" {
		h.t.Fatalf("unexpected host method %s", method)
	}
	key := params.(map[string]any)["key"].(string)
	if !strings.HasSuffix(key, "-movie-"+h.id+"-all") {
		h.t.Fatalf("wrong TMDB key: %s", key)
	}
	b, _ := json.Marshal(receipt{Until: time.Now().Add(time.Hour).Unix(), Result: pluginapi.Result{Message: "cached correct TMDB ID"}})
	return json.Unmarshal(b, out)
}
func TestDianSearchAcceptsNumericTMDBID(t *testing.T) {
	for _, id := range []string{"123", "411442", "999999", "1000000", "1178020", "1195833", "1660900"} {
		for _, representation := range []string{id, `"` + id + `"`} {
			t.Run(representation, func(t *testing.T) {
				var input map[string]any
				if e := json.Unmarshal([]byte(`{"media_type":"movie","tmdb_id":`+representation+`}`), &input); e != nil {
					t.Fatal(e)
				}
				h := &cachedTMDBHost{t: t, id: id}
				result, e := executeOpenAPI(h, pluginapi.Request{ActionID: "search", TaskID: "numeric-id", Config: map[string]any{"email": "fixture@example.test"}, Input: input})
				if e != nil || result.Message != "cached correct TMDB ID" || h.calls != 2 {
					t.Fatalf("TMDB %s failed: %v calls=%d", representation, e, h.calls)
				}
			})
		}
	}
	for _, raw := range []string{`null`, `true`, `0`, `-1`, `1.25`, `"abc"`, `1e99`} {
		var value any
		_ = json.Unmarshal([]byte(raw), &value)
		h := &cachedTMDBHost{t: t}
		_, e := executeOpenAPI(h, pluginapi.Request{ActionID: "search", TaskID: "bad-id", Config: map[string]any{"email": "fixture@example.test"}, Input: map[string]any{"media_type": "movie", "tmdb_id": value}})
		if e == nil || h.calls != 0 {
			t.Fatal(fmt.Sprint("invalid ID accepted: ", raw, e, h.calls))
		}
	}
}
