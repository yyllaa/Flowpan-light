package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flowpan/sdk/pluginwasm"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type liveHTTPHost struct {
	*fixtureHost
	requests int
	report   []map[string]any
}

func (h *liveHTTPHost) Call(method string, in, out any) error {
	if method != "http.fetch" {
		return h.fixtureHost.Call(method, in, out)
	}
	raw, _ := json.Marshal(in)
	var p pluginwasm.HTTPRequest
	_ = json.Unmarshal(raw, &p)
	req, err := http.NewRequestWithContext(context.Background(), p.Method, p.URL, strings.NewReader(p.Body))
	if err != nil {
		return err
	}
	if p.Method == "" {
		req.Method = "GET"
	}
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 35 * time.Second}
	started := time.Now()
	res, err := client.Do(req)
	h.requests++
	if err != nil {
		return fmt.Errorf("HTTP transport %T", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	r := pluginwasm.HTTPResponse{Status: res.StatusCode, Body: string(body), Headers: map[string]string{}}
	if p.ResponseEncoding == "base64" {
		r.Body = base64.StdEncoding.EncodeToString(body)
		r.Encoding = "base64"
	}
	for k, v := range res.Header {
		if len(v) > 0 {
			r.Headers[k] = v[0]
		}
	}
	h.report = append(h.report, map[string]any{"http_status": r.Status, "bytes": len(body), "duration_ms": time.Since(started).Milliseconds(), "kind": map[bool]string{true: "image", false: "metadata"}[p.ResponseEncoding == "base64"]})
	return assign(r, out)
}
func TestMusicLiveMetadataSources(t *testing.T) {
	out := os.Getenv("FLOWPAN_MUSIC_LIVE")
	if out == "" {
		t.Skip("explicit live metadata test")
	}
	h := &liveHTTPHost{fixtureHost: fixture(0)}
	e := &engine{host: h, now: time.Now}
	rows := []map[string]any{}
	usable := 0
	for _, source := range []string{"netease", "qq", "itunes", "musicbrainz"} {
		time.Sleep(4 * time.Second)
		query := tags{}
		query.Title = "晴天"
		query.Artist = "周杰伦"
		request, err := searchRequest(source, query)
		if err != nil {
			t.Fatal(err)
		}
		res, err := e.fetch(source, request)
		row := map[string]any{"source": source, "http_status": res.Status}
		if err != nil {
			row["state"] = "unavailable"
			row["error"] = err.Error()
		} else {
			choices, err := parseSearch(source, res.Body)
			if err != nil {
				row["state"] = "unavailable"
				row["error"] = err.Error()
			} else {
				row["state"] = "ok"
				row["candidates"] = len(choices)
				if len(choices) > 0 {
					usable++
				}
			}
		}
		rows = append(rows, row)
		t.Logf("MUSIC_SOURCE source=%s state=%s http=%d", source, row["state"], res.Status)
	}
	raw, _ := json.MarshalIndent(map[string]any{"sources": rows, "requests": h.report, "usable": usable}, "", "  ")
	if err := os.WriteFile(out, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if usable == 0 {
		t.Fatal("no live metadata source returned candidates")
	}
	t.Logf("MUSIC_LIVE_SOURCES_PASS usable=%d requests=%d", usable, h.requests)
}
