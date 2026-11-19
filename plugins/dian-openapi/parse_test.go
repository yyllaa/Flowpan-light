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

func TestDianOpenAPIEpisodeFieldsReachResourceTitle(t *testing.T) {
	raw := []byte(`{"items":[{"share_id":25,"resource_id":123,"status":"active","media_type":"tv","season":3,"seasons_csv":"3","episodes_csv":"1,2,3,4,5,6","file_name":"The.Lord.of.the.Rings.S03E05.4K.WEB-DL.mkv","total_size_human":"16.42GB"}]}`)
	shares, count, err := parseDianOpenAPIList(raw, "tv")
	if err != nil || count != 1 || len(shares) != 1 {
		t.Fatalf("parse Dian API response: count=%d shares=%d err=%v", count, len(shares), err)
	}
	if got := shares[0].seasonMark(); got != "S03E01-E06" {
		t.Fatalf("season mark = %q, want S03E01-E06", got)
	}
	resources := resourceResult(shares)["resources"].([]resource)
	if len(resources) != 1 || !strings.Contains(resources[0].Title, "[S03E01-E06]") {
		t.Fatalf("Dian resource title = %#v, want API episode range", resources)
	}
}

func TestDianOpenAPIListAcceptsEpisodeArraysAndMovieFieldsStayOutOfEpisodeMark(t *testing.T) {
	raw := []byte(`{"items":[{"share_id":26,"resource_id":124,"status":"active","media_type":"tv","season_number":3,"seasons_csv":[3],"episodes_csv":[1,2,3,4,5,6],"file_name":"The.Lord.of.the.Rings.S03E05.4K.WEB-DL.mkv"},{"share_id":27,"resource_id":125,"status":"active","media_type":"movie","season":3,"seasons_csv":"3","episodes_csv":"1,2,3,4,5,6","title_override":"Movie.2022.4K.WEB-DL"}]}`)
	shares, count, err := parseDianOpenAPIList(raw, "tv")
	if err != nil || count != 2 || len(shares) != 2 {
		t.Fatalf("parse Dian API response: count=%d shares=%d err=%v", count, len(shares), err)
	}
	if got := shares[0].seasonMark(); got != "S03E01-E06" {
		t.Fatalf("array episode mark = %q, want S03E01-E06", got)
	}
	resources := resourceResult(shares)["resources"].([]resource)
	if !strings.Contains(resources[0].Title, "[S03E01-E06]") {
		t.Fatalf("TV resource title = %q, want API episode range", resources[0].Title)
	}
	if strings.Contains(resources[1].Title, "S03") {
		t.Fatalf("movie title acquired TV episode metadata: %q", resources[1].Title)
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
